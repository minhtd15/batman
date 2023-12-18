package store

import (
	"context"
	"database/sql"
	batman "education-website"
	api_request "education-website/api/request"
	api_response "education-website/api/response"
	"education-website/commonconstant"
	"education-website/entity/course_class"
	"education-website/entity/salary"
	"education-website/entity/student"
	"education-website/entity/user"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

type userManagementStore struct {
	db *sqlx.DB
}

type UserManagementStoreCfg struct {
	Db *sqlx.DB
}

func NewUserManagementStore(userManagementStoreCfg UserManagementStoreCfg) *userManagementStore {
	return &userManagementStore{
		db: userManagementStoreCfg.Db,
	}
}

func (u *userManagementStore) GetByUserNameStore(userName string, email string, userId string, ctx context.Context) (batman.UserResponse, error) {
	log.Infof("Get user information by UserName")

	entity := batman.UserResponse{}
	sqlQuery := "SELECT * FROM USER WHERE USERNAME = ? OR EMAIL = ? OR USER_ID = ?"

	// execute sql query
	err := u.db.QueryRowxContext(ctx, sqlQuery, userName, email, userId).Scan(&entity.UserId, &entity.UserName, &entity.Email, &entity.Role, &entity.DOB, &entity.StartDate, &entity.JobPosition, &entity.Password, &entity.FullName, &entity.Gender)
	if err != nil {
		if err == sql.ErrNoRows {
			//log.WithError(err).Errorf("Cannot find user with user name: %s", userName)
			return entity, nil
		}
		log.WithError(err).Errorf("Cannot get info from database for user: %s", userName)
		return entity, err
	}
	return entity, nil
}

func (u *userManagementStore) InsertNewUserStore(newUser user.UserEntity, ctx context.Context) error {
	log.Infof("insert new user to database after validated register information")

	// Start a new transaction
	tx, err := u.db.Begin()
	if err != nil {
		log.WithError(err).Errorf("Failed to begin transaction")
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			// Something went wrong, rollback the transaction
			tx.Rollback()
			panic(p) // Re-throw the panic after rollback
		} else if err != nil {
			// Error occurred, rollback the transaction
			tx.Rollback()
		}
	}()

	sqlQuery := "INSERT INTO USER VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	stmt, err := tx.Prepare(sqlQuery)
	if err != nil {
		log.WithError(err).Errorf("Failed to prepare SQL statement")
		return err
	}
	defer stmt.Close()

	// Execute the prepared statement
	result, err := stmt.Exec(newUser.UserId, newUser.UserName, newUser.Email, newUser.Role, newUser.DOB, newUser.StartingDate, newUser.JobPosition, newUser.Password, newUser.FullName, newUser.Gender)
	if err != nil {
		log.WithError(err).Errorf("Failed to insert user into the database")
		return err
	}

	// Get the last insert ID
	_, err = result.LastInsertId()
	if err != nil {
		log.WithError(err).Errorf("Failed to get last insert ID")
		return err
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		log.WithError(err).Errorf("Failed to commit transaction")
		return err
	}

	return nil
}

func (u *userManagementStore) UpdateNewPassword(newPassword []byte, userName string) error {
	log.Infof("Start to update new password")

	// Begin a transaction
	tx, err := u.db.Begin()
	if err != nil {
		log.WithError(err).Errorf("Failed to begin transaction")
		return err
	}

	defer func() {
		// Rollback the transaction if there is an error or return is not nil
		if r := recover(); r != nil || err != nil {
			log.WithError(err).Errorf("Rolling back transaction")
			tx.Rollback()
			return
		}
		// Commit the transaction if there is no error
		err := tx.Commit()
		if err != nil {
			log.WithError(err).Errorf("Failed to commit transaction")
		}
	}()

	sqlQuery := "UPDATE USER SET PASSWORD = ? WHERE USERNAME = ?"

	stmt, err := tx.Prepare(sqlQuery)
	if err != nil {
		log.WithError(err).Errorf("Failed to prepare SQL statement")
		return err
	}
	defer stmt.Close()

	// Execute the prepared statement within the transaction
	_, err = stmt.Exec(newPassword, userName)
	if err != nil {
		log.WithError(err).Errorf("Failed to update user password in the database")
		return err
	}

	// Return nil if the update is successful
	return nil
}

func (u *userManagementStore) GetSalaryReportStore(userName string, month string, year string, ctx context.Context) ([]salary.SalaryEntity, error) {
	log.Infof("Get salary information from database")

	var entities []salary.SalaryEntity

	var sqlQuery string
	var rows *sqlx.Rows
	sqlQuery = "select u.USER_ID, u.USERNAME, u.FULLNAME, u.GENDER, u.JOB_POSITION, s.PAYROLL_ID, s.TYPE_PAYROLL, s.TOTAL_WORK_DATES, s.PAYROLL_RATE, s.SALARY " +
		"from SALARY s join USER u ON s.USER_ID = u.USER_ID " +
		"WHERE s.MONTH = ? and s.YEAR = ?"
	args := []interface{}{month, year}

	if userName != "" {
		sqlQuery += " AND u.FULLNAME LIKE CONCAT('%', ?, '%')"
		args = append(args, userName)
	}

	rows, err := u.db.QueryxContext(ctx, sqlQuery, args...)
	if err != nil {
		log.WithError(err).Errorf("Cannot get info from the database for user: %s", userName)
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var entity salary.SalaryEntity
		if err := rows.Scan(&entity.UserId, &entity.UserName, &entity.FullName, &entity.Gender, &entity.JobPosition, &entity.PayrollId, &entity.TypeWork, &entity.TotalWorkDates, &entity.PayrollPerSessions, &entity.TotalSalary); err != nil {
			log.WithError(err).Errorf("Error scanning row: %s", err.Error())
			return nil, err
		}
		entities = append(entities, entity)
	}

	if err := rows.Err(); err != nil {
		log.WithError(err).Errorf("Error iterating rows: %s", err.Error())
		return nil, err
	}

	return entities, nil
}

func (u *userManagementStore) ModifySalaryConfigurationStore(userId string, userSalaryInfo []api_request.SalaryConfiguration, ctx context.Context) error {
	log.Infof("modify salary information for user %s", userId)

	sqlQuery := "UPDATE EMPLOYEE_RATE SET PAYROLL_RATE = ? WHERE USER_ID = ? AND PAYROLL_ID = ?"

	for _, entities := range userSalaryInfo {
		_, err := u.db.ExecContext(ctx, sqlQuery, entities.PayrollAmount, userId, entities.PayrollId)
		if err != nil {
			log.WithError(err).Errorf("Failed to update salary configuration for user: %s", userId)
			return err
		}
	}

	log.Infof("Salary configuration updated successfully for user: %s", userId)
	return nil
}

func (u *userManagementStore) InsertStudentStore(data []student.EntityStudent, courseId string, ctx context.Context) error {
	log.Infof("Insert to db students list")

	tx, err := u.db.Begin()
	if err != nil {
		log.WithError(err).Errorf("Failed to begin transaction")
		return err
	}

	defer func() {
		if err != nil {
			log.WithError(err).Errorf("Rolling back transaction")
			tx.Rollback()
			return
		}
		err = tx.Commit()
		if err != nil {
			log.WithError(err).Errorf("Error committing transaction")
		}
	}()

	// Insert into COURSE table
	sqlClass := "INSERT INTO STUDENT (STUDENT_NAME, DOB, EMAIL, PHONE_NUMBER) VALUES (?, ?, ?, ?)"
	stmt, err := tx.Prepare(sqlClass)
	if err != nil {
		log.WithError(err).Errorf("Failed to prepare SQL statement for CLASS")
		return err
	}

	sqlQuery := "INSERT INTO COURSE_MANAGER (STUDENT_ID, COURSE_ID) VALUES (?, ?)"
	tmp, err := tx.Prepare(sqlQuery)
	if err != nil {
		log.WithError(err).Errorf("Failed to prepare SQL statement for CLASS")
		return err
	}
	defer stmt.Close()

	for _, v := range data {
		rs, err := stmt.Exec(v.Name, v.DOB, v.Email, v.PhoneNo)
		if err != nil {
			log.WithError(err).Errorf("Failed to insert class into the database")
			return err
		}

		id, err := rs.LastInsertId()
		if err != nil {
			log.WithError(err).Errorf("Failed to get last insert ID")
			return err
		}

		_, err = tmp.Exec(id, courseId)
		if err != nil {
			log.WithError(err).Errorf("Failed to insert class into the database")
			return err
		}
	}

	return nil
}

func (u *userManagementStore) ModifyUserInformationStore(rq api_request.ModifyUserInformationRequest, userId string, ctx context.Context) error {
	log.Infof("Insert to db students list")

	sqlQuery := "UPDATE USER SET EMAIL = ?, DOB = ?, FULLNAME = ?, GENDER = ? WHERE USER_ID = ?"

	_, err := u.db.ExecContext(ctx, sqlQuery, rq.Email, rq.Dob, rq.FullName, rq.Gender, userId)
	if err != nil {
		log.WithError(err).Errorf("Error update user %s information on db", userId)
		return err
	}

	log.Infof("Successful update user information")
	return nil
}

func (u *userManagementStore) InsertOneStudentStore(rq api_request.NewStudentRequest, courseId string, ctx context.Context) error {
	log.Infof("Insert one student to database")

	dob, err := time.Parse("2006-01-02", rq.DOB)
	if err != nil {
		log.WithError(err).Errorf("Unable to parse dob for student: %s", err)
		return err
	}

	tmp := student.EntityStudent{
		Name:    rq.Name,
		DOB:     dob,
		Email:   rq.Email,
		PhoneNo: rq.PhoneNumber,
	}

	tx, err := u.db.Begin()
	if err != nil {
		log.WithError(err).Errorf("Failed to begin transaction")
		return err
	}

	defer func() {
		if err != nil {
			log.WithError(err).Errorf("Rolling back transaction")
			tx.Rollback()
			return
		}
		err = tx.Commit()
		if err != nil {
			log.WithError(err).Errorf("Error committing transaction")
		}
	}()

	// Insert into STUDENT table
	sqlStudent := "INSERT INTO STUDENT (STUDENT_NAME, DOB, EMAIL, PHONE_NUMBER) VALUES (?, ?, ?, ?)"
	stmtStudent, err := tx.Prepare(sqlStudent)
	if err != nil {
		log.WithError(err).Errorf("Failed to prepare SQL statement for STUDENT")
		return err
	}
	defer stmtStudent.Close()

	result, err := stmtStudent.Exec(tmp.Name, tmp.DOB, tmp.Email, tmp.PhoneNo)
	if err != nil {
		log.WithError(err).Errorf("Failed to insert student into the database")
		return err
	}

	// Get the last insert ID
	lastInsertID, err := result.LastInsertId()
	if err != nil {
		log.WithError(err).Errorf("Failed to get last insert ID")
		return err
	}

	// Add another SQL query
	// For example, insert into ANOTHER_TABLE
	sqlAnotherTable := "INSERT INTO COURSE_MANAGER (STUDENT_ID, COURSE_ID) VALUES (?, ?)"
	stmtAnotherTable, err := tx.Prepare(sqlAnotherTable)
	if err != nil {
		log.WithError(err).Errorf("Failed to prepare SQL statement for ANOTHER_TABLE")
		return err
	}
	defer stmtAnotherTable.Close()
	_, err = stmtAnotherTable.Exec(lastInsertID, courseId) // Replace with actual values
	if err != nil {
		log.WithError(err).Errorf("Failed to insert data into ANOTHER_TABLE")
		return err
	}

	return nil
}

func (u *userManagementStore) CheckCourseExistence(courseId string, ctx context.Context) error {
	log.Infof("Start to check course existence in db store")

	var count int
	sqlQuery := "SELECT COUNT(*) FROM COURSE WHERE COURSE_ID = ?"
	err := u.db.GetContext(ctx, &count, sqlQuery, courseId)
	if err != nil {
		log.WithError(err).Errorf("Error checking existence for course %s", courseId)
		return err
	}

	if count == 0 {
		log.Infof("Course %s does not exist", courseId)
		return commonconstant.ErrCourseNotExist
	}

	log.Infof("Course %s exists", courseId)
	return nil
}

func (u *userManagementStore) GetUserByJobPosition(jobPos string, ctx context.Context) ([]user.UserEntity, error) {
	log.Infof("Start to get user by job position")

	var entities []user.UserEntity
	sqlQuery := "SELECT * FROM USER WHERE JOB_POSITION = ?"

	// Execute the SQL query
	rows, err := u.db.QueryContext(ctx, sqlQuery, jobPos)
	if err != nil {
		log.Errorf("Error executing SQL query: %v", err)
		return nil, err
	}
	defer rows.Close()

	// Iterate through the result set and scan into UserEntity structs
	for rows.Next() {
		var entity user.UserEntity
		err := rows.Scan(
			&entity.UserId,
			&entity.UserName,
			&entity.Email,
			&entity.Role,
			&entity.DOB,
			&entity.JobPosition,
			&entity.StartingDate,
			&entity.Password,
			&entity.Gender,
			&entity.FullName,
		)
		if err != nil {
			log.Errorf("Error scanning row: %v", err)
			return nil, err
		}
		entities = append(entities, entity)
	}

	if err := rows.Err(); err != nil {
		log.Errorf("Error iterating through result set: %v", err)
		return nil, err
	}

	log.Infof("Successfully retrieved users by job position")
	return entities, nil
}

func (u *userManagementStore) GetUserSalaryConfigStore(userId string, ctx context.Context) ([]api_response.SalaryConfig, error) {
	log.Infof("Start to get user salary config")

	var entities []api_response.SalaryConfig
	args := []interface{}{userId}
	sqlQuery := "SELECT ER.USER_ID, ER.PAYROLL_ID, P.TYPE_PAYROLL, ER.PAYROLL_RATE FROM EMPLOYEE_RATE ER JOIN PAYROLL P ON ER.PAYROLL_ID = P.PAYROLL_ID WHERE USER_ID = ?"

	rows, err := u.db.QueryxContext(ctx, sqlQuery, args...)
	if err != nil {
		log.WithError(err).Errorf("Cannot get info from the database for user: %s", userId)
		return nil, err
	}

	defer rows.Close()

	var tmpUserId string
	for rows.Next() {
		var entity api_response.SalaryConfig
		if err := rows.Scan(&tmpUserId, &entity.PayrollId, &entity.TypePayroll, &entity.PayrollRate); err != nil {
			log.WithError(err).Errorf("Error scanning row: %s", err.Error())
			return nil, err
		}
		entities = append(entities, entity)
	}

	if err := rows.Err(); err != nil {
		log.WithError(err).Errorf("Error iterating rows: %s", err.Error())
		return nil, err
	}
	return entities, nil
}

func (u *userManagementStore) GetStudentByCourseIdStore(courseId string, ctx context.Context) ([]student.EntityStudent, error) {
	log.Infof("Get student by course Id store")

	var entities []student.EntityStudent
	var sqlQuery string
	var rows *sqlx.Rows
	sqlQuery = "SELECT S.* FROM STUDENT S JOIN COURSE_MANAGER CM ON S.STUDENT_ID = CM.STUDENT_ID WHERE CM.COURSE_ID = ?"
	args := []interface{}{courseId}

	rows, err := u.db.QueryxContext(ctx, sqlQuery, args...)
	if err != nil {
		log.WithError(err).Errorf("Cannot get info from the database for course: %s", courseId)
		return nil, err
	}

	defer rows.Close()
	var dob string

	for rows.Next() {
		var entity student.EntityStudent
		if err := rows.Scan(&entity.Id, &entity.Name, &dob, &entity.Email, &entity.PhoneNo); err != nil {
			log.WithError(err).Errorf("Error scanning row: %s", err.Error())
			return nil, err
		}
		entity.DOB, err = time.Parse("2006-01-02", dob)
		if err != nil {
			log.WithError(err).Errorf("Parse time error")
			return nil, err
		}
		entities = append(entities, entity)
	}

	if err := rows.Err(); err != nil {
		log.WithError(err).Errorf("Error iterating rows: %s", err.Error())
		return nil, err
	}

	return entities, nil
}

func (u *userManagementStore) AddStudentAttendanceStore(rq student.StudentAttendanceEntity, ctx context.Context) error {
	log.Infof("POST student attendance store, classId %s", rq.ClassId)

	// Start a new transaction
	tx, err := u.db.Begin()
	if err != nil {
		log.WithError(err).Errorf("Failed to start a transaction")
		return err
	}
	defer func() {
		// Check if there was an error, if so, rollback the transaction
		if err != nil {
			log.WithError(err).Errorf("Rolling back the transaction")
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				log.WithError(rollbackErr).Errorf("Failed to rollback the transaction")
			}
		}
	}()

	sqlQuery := "INSERT INTO STUDENT_ATTENDANCE (STUDENT_ID, CLASS_ID, STATUS) VALUES (?, ?, ?)"
	stmt, err := tx.Prepare(sqlQuery)
	if err != nil {
		log.WithError(err).Errorf("Failed to prepare SQL statement")
		return err
	}
	defer stmt.Close()

	// Execute the prepared statement within the transaction
	_, err = stmt.Exec(rq.StudentId, rq.ClassId, rq.Status)
	if err != nil {
		log.WithError(err).Errorf("Failed to add student attendance into the database")
		return err
	}

	// If everything is successful, commit the transaction
	if err := tx.Commit(); err != nil {
		log.WithError(err).Errorf("Failed to commit the transaction")
		return err
	}

	return nil
}

func (u *userManagementStore) GetScheduleByCourseIdStore(studentList []student.EntityStudent, courseId string, ctx context.Context) ([]api_response.StudentAttendanceScheduleResponse, error) {
	log.Infof("Get student schedule for %s", courseId)

	var rs []api_response.StudentAttendanceScheduleResponse

	sqlQuery := "SELECT SA.CLASS_ID, SA.STATUS, C.DATE FROM STUDENT_ATTENDANCE SA JOIN CLASS C ON C.CLASS_ID = SA.CLASS_ID WHERE STUDENT_ID = ? "

	for _, v := range studentList {
		id, _ := strconv.ParseInt(v.Id, 10, 64)
		tmp := api_response.StudentAttendanceScheduleResponse{
			StudentId: id,
			Name:      v.Name,
			Dob:       v.DOB.Format("2006-01-02"),
		}

		rows, err := u.db.QueryContext(ctx, sqlQuery, id)
		if err != nil {
			log.Errorf("Error executing SQL query: %v", err)
			return nil, err
		}
		defer rows.Close()

		var entities []api_response.CheckInStudent
		// Iterate through the result set and scan into UserEntity structs
		for rows.Next() {
			var entity api_response.CheckInStudent
			err := rows.Scan(
				&entity.ClassId,
				&entity.Status,
				&entity.Date,
			)
			if err != nil {
				log.Errorf("Error scanning row: %v", err)
				return nil, err
			}
			entities = append(entities, entity)
		}

		tmp.CheckIn = entities
		rs = append(rs, tmp)
	}

	log.Infof("Successfull get data :%s", rs)
	return rs, nil
}

func (u *userManagementStore) UpdateStudentAttendanceStore(rq api_request.StudentAttendanceRequest, ctx context.Context) error {
	log.Infof("Start to update new student %s attendance", rq.StudentId)

	// Begin a transaction
	tx, err := u.db.Begin()
	if err != nil {
		log.WithError(err).Errorf("Failed to begin transaction")
		return err
	}

	defer func() {
		// Rollback the transaction if there is an error or return is not nil
		if r := recover(); r != nil || err != nil {
			log.WithError(err).Errorf("Rolling back transaction")
			tx.Rollback()
			return
		}
		// Commit the transaction if there is no error
		err := tx.Commit()
		if err != nil {
			log.WithError(err).Errorf("Failed to commit transaction")
		}
	}()

	sqlQuery := "UPDATE STUDENT_ATTENDANCE SET STATUS = ? WHERE STUDENT_ID = ? AND CLASS_ID = ?"

	stmt, err := tx.Prepare(sqlQuery)
	if err != nil {
		log.WithError(err).Errorf("Failed to prepare SQL statement")
		return err
	}
	defer stmt.Close()

	// Execute the prepared statement within the transaction
	_, err = stmt.Exec(rq.Status, rq.StudentId, rq.ClassId)
	if err != nil {
		log.WithError(err).Errorf("Failed to update user password in the database")
		return err
	}

	// Return nil if the update is successful
	return nil
}

func (u *userManagementStore) GetUserCourseInChargeStore(username string, ctx context.Context) ([]course_class.CourseEntity, error) {
	log.Infof("Start to get user courses in charge store", username)
	var entities []course_class.CourseEntity
	sqlQuery := "SELECT C.*, CONCAT(CT.CODE, C.COURSE_ID) AS COURSE_NAME FROM COURSE C JOIN COURSE_TYPE CT ON C.COURSE_TYPE_ID = CT.COURSE_TYPE_ID WHERE C.MAIN_TEACHER = ?"

	// Execute the SQL query
	rows, err := u.db.QueryContext(ctx, sqlQuery, username)
	if err != nil {
		log.Errorf("Error executing SQL query: %v", err)
		return nil, err
	}
	defer rows.Close()

	// Iterate through the result set and scan into UserEntity structs
	for rows.Next() {
		var entity course_class.CourseEntity
		err := rows.Scan(
			&entity.CourseId,
			&entity.CourseTypeId,
			&entity.MainTeacher,
			&entity.Room,
			&entity.StartDate,
			&entity.EndDate,
			&entity.StartTime,
			&entity.EndTime,
			&entity.StudyDays,
			&entity.Location,
			&entity.CourseName,
		)
		if err != nil {
			log.Errorf("Error scanning row: %v", err)
			return nil, err
		}
		entities = append(entities, entity)
	}

	if err := rows.Err(); err != nil {
		log.Errorf("Error iterating through result set: %v", err)
		return nil, err
	}

	log.Infof("Successfully get all course that user %s in charge", username)
	return entities, nil
}
