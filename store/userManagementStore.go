package store

import (
	"context"
	"database/sql"
	batman "education-website"
	api_request "education-website/api/request"
	"education-website/commonconstant"
	"education-website/entity/salary"
	"education-website/entity/student"
	"education-website/entity/user"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
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

	sqlQuery := "INSERT INTO USER VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	stmt, err := u.db.Prepare(sqlQuery)
	if err != nil {
		log.WithError(err).Errorf("Failed to prepare SQL statement")
		return err
	}
	defer stmt.Close()

	// Execute the prepared statement
	_, err = stmt.Exec(newUser.UserId, newUser.UserName, newUser.Email, newUser.Role, newUser.DOB, newUser.StartingDate, newUser.JobPosition, newUser.Password, newUser.FullName, newUser.Gender)
	if err != nil {
		log.WithError(err).Errorf("Failed to insert user into the database")
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

func (u *userManagementStore) InsertOneStudentStore(rq api_request.NewStudentRequest, ctx context.Context) error {
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

	// Insert into COURSE table
	sqlClass := "INSERT INTO STUDENT (STUDENT_NAME, DOB, EMAIL, PHONE_NUMBER) VALUES (?, ?, ?, ?)"
	stmt, err := tx.Prepare(sqlClass)
	if err != nil {
		log.WithError(err).Errorf("Failed to prepare SQL statement for CLASS")
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(tmp.Name, tmp.DOB, tmp.Email, tmp.PhoneNo)
	if err != nil {
		log.WithError(err).Errorf("Failed to insert class into the database")
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
