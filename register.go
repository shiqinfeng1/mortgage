package main

import (
	"database/sql"
	_ "errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	_ "log"
	_ "os"
)

type SqlType struct {
	db *sql.DB
}

func ConnectDB(dbname string) (db *sql.DB, err error) {

	//db, err = sql.Open("mysql", "root:123456@tcp(127.0.0.1:3306)/mydb?charset=utf8")
	mydbname := "root:12345678@tcp(127.0.0.1:3306)/" + dbname + "?charset=utf8"
	db, err = sql.Open("mysql", mydbname)
	if !checkErr(err) {
		return
	}
	err = db.Ping()
	if !checkErr(err) {
		return
	}
	//check := "SELECT table_name FROM information_schema.TABLES WHERE table_name ='"+dbname+"' "

	sqls := []string{
		"set names utf8",
		"CREATE TABLE user (id INT auto_increment, user_name CHAR(50) UNIQUE NOT NULL DEFAULT '', user_passwd CHAR(128) NOT NULL DEFAULT '', bankcard CHAR(64) NOT NULL DEFAULT '', file1 CHAR(128) NOT NULL DEFAULT '', file2 CHAR(128) NOT NULL DEFAULT '', file3 CHAR(128) NOT NULL DEFAULT '', verify_ok INT NOT NULL DEFAULT 0, realy_name CHAR(128) NOT NULL DEFAULT '', timestamp CHAR(128) NOT NULL DEFAULT '', primary key(id))",
		"CREATE TABLE admin (id INT auto_increment, admin_name CHAR(50) UNIQUE NOT NULL DEFAULT '', admin_passwd CHAR(128) NOT NULL DEFAULT '', primary key(id))",
		"CREATE TABLE asset (asset_account CHAR(64) NOT NULL DEFAULT '', user_name CHAR(50) NOT NULL DEFAULT '', asset_type CHAR(32) NOT NULL DEFAULT '', primary key(asset_account))",
		"CREATE TABLE apply (id INT auto_increment, user_name CHAR(50) NOT NULL DEFAULT '', asset_account CHAR(128) NOT NULL DEFAULT '', status INT NOT NULL DEFAULT 0, timestamp CHAR(128) NOT NULL DEFAULT '', duration INT NOT NULL DEFAULT 0, money INT NOT NULL DEFAULT 0, type CHAR(32) NOT NULL DEFAULT '', total_amount DOUBLE NOT NULL DEFAULT 0.0, total_interest DOUBLE NOT NULL DEFAULT 0.0, primary key(id))",
		"CREATE TABLE record (id INT auto_increment, user_name CHAR(50) NOT NULL DEFAULT '', asset_account CHAR(128) NOT NULL DEFAULT '', apply_id INT NOT NULL DEFAULT 0, timestamp CHAR(128) NOT NULL DEFAULT '', price DOUBLE NOT NULL DEFAULT 0.0, amount INT NOT NULL DEFAULT 0, interest DOUBLE NOT NULL DEFAULT 0.0, primary key(id))",
		"alter table user change realy_name realy_name char(100) character set utf8 collate utf8_unicode_ci not null default ''",
	}
	for _, sql := range sqls {
		if _, err := db.Exec(sql); err != nil {
			
			if w, ok := err.(mysql.MySQLWarnings); ok {
				fmt.Printf("WARING on %q: %v\n", sql, w)
			} else {
				//fmt.Printf("ERROR on %q: %v\n", sql, err)
			}
			
		}
	}
	return 
}

//check unique
func CheckUnique(db *sql.DB, userName string) (bl bool, err error) {

	rows, err := db.Query("select * from user where user_name=?", userName)
	defer rows.Close()
	if checkErr(err) != true {
		bl = false
		return
	}

	if false != rows.Next() {
		fmt.Printf("user %v alrealdy exists",userName)
		return false, nil
	}
	return true, nil
}

func CheckAdminIsExist(db *sql.DB, adminname string) (bl bool, err error) {

	rows, err := db.Query("select * from admin where admin_name=?", adminname)
	defer rows.Close()

	if !checkErr(err) {
		return false, err
	}

	if rows.Next() {
		return true, nil
	}
	return false, nil
}

func CheckIsExist(db *sql.DB, username string) (bl bool, err error) {

	rows, err := db.Query("select * from user where user_name=?", username)
	defer rows.Close()

	if !checkErr(err) {
		return false, err
	}

	if rows.Next() {
		return true, nil
	}
	return false, nil
}

func GetAdminPasswd(db *sql.DB, userName string) (passwd string, err error) {

	var password string

	rows, err := db.Query("select admin_passwd from admin where admin_name=?", userName)
	defer rows.Close()

	if !checkErr(err) {
		return password, err
	}

	if rows.Next() {
		err = rows.Scan(&password)
		if !checkErr(err) {
			return password, err
		}
	}
	return password, nil
}
func GetUserPasswd(db *sql.DB, userName string) (passwd string, err error) {

	var password string

	rows, err := db.Query("select user_passwd from user where user_name=?", userName)
	defer rows.Close()

	if !checkErr(err) {
		return password, err
	}

	if rows.Next() {
		err = rows.Scan(&password)
		if !checkErr(err) {
			return password, err
		}
	}
	return password, nil
}
func UpdateVerify(db *sql.DB, userName string, verify_ok int) (err error) {
	stmt, err := db.Prepare("update user set verify_ok=? where user_name=?")
	defer stmt.Close()
	checkErr(err)

	if _, err := stmt.Exec(verify_ok, userName); err != nil {
		if !checkErr(err) {
			return err
		}
	}
	fmt.Printf("%s update verify_ok:%d\n", userName,verify_ok)
	return nil
}
func UpdateBankCard(db *sql.DB, userName string, bankcard string, realname string) (err error) {
	stmt, err := db.Prepare("update user set bankcard=?,realy_name=? where user_name=?")
	defer stmt.Close()
	checkErr(err)

	if _, err := stmt.Exec(bankcard, realname, userName); err != nil {
		if !checkErr(err) {
			return err
		}
	}
	fmt.Printf("%s update bankcard:%s\n", userName,bankcard)
	return nil
}

func DelBankCard(db *sql.DB, userName string, bankcard string) (err error) {
	stmt, err := db.Prepare("update user set bankcard=? where user_name=?")
	defer stmt.Close()
	checkErr(err)

	if _, err := stmt.Exec("", userName); err != nil {
		if !checkErr(err) {
			return err
		}
	}
	fmt.Printf("%s delete bankcard:%s\n", userName,bankcard)
	return nil
}
func UpdateImages(db *sql.DB, userName string, index string, filename string) (err error) {
	fileindex := "file"+index
	stmt, err := db.Prepare("update user set "+fileindex+"=? where user_name=?")
	defer stmt.Close()
	checkErr(err)
	if _, err := stmt.Exec(filename, userName); err != nil {
		if !checkErr(err) {
			return err
		}
	}
	fmt.Printf("%s update image:%s fileidx:%s\n", userName,filename,fileindex)
	return nil
}

func InsertPassword(db *sql.DB, userName string, userPasswd string,timestamp string) (err error) {

	stmt, err := db.Prepare(`INSERT user (user_name,user_passwd,timestamp) values (?,?,?)`)
	defer stmt.Close()
	if !checkErr(err) {
		return
	}
	res, err := stmt.Exec(userName, userPasswd,timestamp)
	if !checkErr(err) {
		return
	}
	lastid, _ := res.LastInsertId()
	fmt.Println(lastid)
	return nil
}

func InsertUserAsset(db *sql.DB, asset *Asset) (err error) {

	stmt, err := db.Prepare(`INSERT asset (asset_account,user_name,asset_type) values (?,?,?)`)
	defer stmt.Close()
	if !checkErr(err) {
		return
	}
	res, err := stmt.Exec(asset.Account, asset.Phonenum,asset.Accounttype)
	if !checkErr(err) {
		return
	}
	lastid, _ := res.LastInsertId()
	fmt.Println(lastid)
	return nil
}
func DelAssetAccount(db *sql.DB, account string) (err error) {
	stmt, err := db.Prepare("delete from asset where asset_account=?")
	defer stmt.Close()
	checkErr(err)

	if _, err := stmt.Exec(account); err != nil {
		if !checkErr(err) {
			return err
		}
	}
	fmt.Printf("delete asset account:%s\n",account)
	return nil
}
func InsertApply(db *sql.DB, apply *ApplyInfo) (err error) {

	stmt, err := db.Prepare(`INSERT apply (user_name,asset_account,timestamp,duration,money,type) values (?,?,?,?,?,?)`)
	defer stmt.Close()
	if !checkErr(err) {
		return
	}
	res, err := stmt.Exec(apply.Phonenum,apply.Account,apply.Timestamp,apply.Duration,apply.Moneyamount,apply.Accounttype)
	if !checkErr(err) {
		return
	}
	lastid, _ := res.LastInsertId()
	fmt.Printf("lastid: %d\n", lastid)
	return nil
}

func InsertRecord(db *sql.DB, record *RecordInfo) (err error) {

	stmt, err := db.Prepare(`INSERT record (user_name,asset_account,apply_id,timestamp,price,amount,interest) values (?,?,?,?,?,?,?)`)
	defer stmt.Close()
	if !checkErr(err) {
		return
	}
	res, err := stmt.Exec(record.Phonenum,record.Account,record.Applyid,record.Timestamp,record.Price,record.Tokenamount,record.Interest)
	if !checkErr(err) {
		return
	}
	lastid, _ := res.LastInsertId()
	fmt.Println(lastid)
	return nil
}
func QueryUserInfoAll(db *sql.DB, currentPage, perPage int) (userinfo []UserInfo, total int, err error){
	var (
		id, sum, fromIdx int
		passwd   string
		temp UserInfo
	)

	rows, err := db.Query("select * from user")
	defer rows.Close()

	if !checkErr(err) {
		return []UserInfo{}, sum, err
	}
	
	for rows.Next() {
		err := rows.Scan(&id, &temp.Phonenum, &passwd, 
			&temp.Bankcard,&temp.Path1,&temp.Path2,&temp.Path3,
			&temp.Verifyok,&temp.Realname,&temp.Timestamp)
		if !checkErr(err) {
			return []UserInfo{}, 0, err
		}
		userinfo = append(userinfo, temp)
	}
	if currentPage == 0 {
		total = len(userinfo)
		return 
	}
	total = len(userinfo)
	fromIdx = (currentPage - 1) * perPage
	if len(userinfo) > fromIdx + perPage {
		userinfo = userinfo[fromIdx:fromIdx+perPage]
	} else if len(userinfo) > fromIdx {
		userinfo = userinfo[fromIdx:]
	}else{
		userinfo = []UserInfo{}
	}
	return 
}
func QueryUserInfo(db *sql.DB, userName string) (userinfo UserInfo,err error){
	var (
		id int
		passwd   string
	)

	rows, err := db.Query("select * from user where user_name=?",userName)
	defer rows.Close()

	if !checkErr(err) {
		return UserInfo{}, err
	}
	
	for rows.Next() {
		err := rows.Scan(&id, &userinfo.Phonenum, &passwd, 
			&userinfo.Bankcard,&userinfo.Path1,&userinfo.Path2,&userinfo.Path3,
			&userinfo.Verifyok,&userinfo.Realname,&userinfo.Timestamp)
		if !checkErr(err) {
			return UserInfo{}, err
		}
	}
	return 
}

func QueryUserAsset(db *sql.DB, userName string) (asset []Asset,err error){

	var temp Asset

	rows, err := db.Query("select * from asset where user_name=?",userName)
	defer rows.Close()

	if !checkErr(err) {
		return []Asset{}, err
	}
	
	for rows.Next() {
		err := rows.Scan(&temp.Account, &temp.Phonenum, &temp.Accounttype)
		if !checkErr(err) {
			return []Asset{}, err
		}
		asset = append(asset, temp)
	}
	return 
}
func QueryUserApplyAll(db *sql.DB, currentPage, perPage int) (applyinfo []ApplyInfo,total int,err error){
	var (
		temp ApplyInfo
	)
	rows, err := db.Query("select * from apply order by status asc")
	defer rows.Close()

	if !checkErr(err) {
		return []ApplyInfo{}, 0,err
	}

	for rows.Next() {
		err := rows.Scan(&temp.Applyid,&temp.Phonenum, &temp.Account, &temp.Status,
			&temp.Timestamp,&temp.Duration,&temp.Moneyamount,&temp.Accounttype,
			&temp.Totalamount,&temp.Totalinterest)
		if !checkErr(err) {
			return []ApplyInfo{}, 0,err
		}
		applyinfo = append(applyinfo, temp)
	}
	if currentPage == 0 {
		total = len(applyinfo)
		return 
	}
	total = len(applyinfo)
	fromIdx := (currentPage - 1) * perPage
	if len(applyinfo) > fromIdx + perPage {
		applyinfo = applyinfo[fromIdx:fromIdx+perPage]
	} else if len(applyinfo) > fromIdx {
		applyinfo = applyinfo[fromIdx:]
	}else{
		applyinfo = []ApplyInfo{}
	}
	return 
}
func QueryUserApply(db *sql.DB, username string) (applyinfo []ApplyInfo,err error){
	var (
		temp ApplyInfo
	)
	rows, err := db.Query("select * from apply where user_name=?", username) // order by status asc
	defer rows.Close()

	if !checkErr(err) {
		return []ApplyInfo{}, err
	}

	for rows.Next() {
		err := rows.Scan(&temp.Applyid,&temp.Phonenum, &temp.Account, &temp.Status,
			&temp.Timestamp,&temp.Duration,&temp.Moneyamount,&temp.Accounttype,
			&temp.Totalamount,&temp.Totalinterest)
		if !checkErr(err) {
			return []ApplyInfo{}, err
		}
		userinfo, err := QueryUserInfo(db, temp.Phonenum)
		temp.Bankcard = userinfo.Bankcard
		applyinfo = append(applyinfo, temp)
	}
	return 
}

func QueryUserApplyById(db *sql.DB, id int) (applyinfo ApplyInfo,err error){
	var (
		id_local int
	)
	rows, err := db.Query("select * from apply where id=?",id)
	defer rows.Close()

	if !checkErr(err) {
		return ApplyInfo{}, err
	}

	for rows.Next() {
		err := rows.Scan(&id_local,&applyinfo.Phonenum, &applyinfo.Account, &applyinfo.Status,
			&applyinfo.Timestamp,&applyinfo.Duration,&applyinfo.Moneyamount,&applyinfo.Accounttype,
			&applyinfo.Totalamount,&applyinfo.Totalinterest)
		if !checkErr(err) {
			return ApplyInfo{}, err
		}
	}
	return 
}

func UpdateApplyStatus(db *sql.DB, id int, status int) (err error) {
	stmt, err := db.Prepare("update apply set status=? where id=?")
	defer stmt.Close()
	checkErr(err)

	if _, err := stmt.Exec(status, id); err != nil {
		if !checkErr(err) {
			return err
		}
	}
	fmt.Printf("id=%d update apply status:%d\n", id,status)
	return nil
}

func UpdateApplyTotalAmount(db *sql.DB, id int, totalamount float64, totalinterest float64) (err error) {
	stmt, err := db.Prepare("update apply set total_amount=?,total_interest=? where id=?")
	defer stmt.Close()
	checkErr(err)

	if _, err := stmt.Exec(totalamount, totalinterest, id); err != nil {
		if !checkErr(err) {
			return err
		}
	}
	fmt.Printf("id=%d update apply total amount:%f interest:%f\n", id,totalamount,totalinterest)
	return nil
}
func QueryUserRecord(db *sql.DB, applyid, currentPage, perPage int) (recordInfo []RecordInfo,total int,err error){
	var (
		temp RecordInfo
	)
	rows, err := db.Query("select * from record where apply_id=?",applyid)
	defer rows.Close()

	if !checkErr(err) {
		return []RecordInfo{}, 0, err
	}
	for rows.Next() {
		err := rows.Scan(&temp.Applyid,&temp.Phonenum, &temp.Account, &temp.Applyid,
			&temp.Timestamp,&temp.Price,&temp.Tokenamount,&temp.Interest)
		if !checkErr(err) {
			return []RecordInfo{}, 0, err
		}
		recordInfo = append(recordInfo, temp)
	}
	if currentPage == 0 {
		total = len(recordInfo)
		return 
	}
	fromIdx := (currentPage - 1) * perPage
	total = len(recordInfo)
	if len(recordInfo) > fromIdx + perPage {
		recordInfo = recordInfo[fromIdx:fromIdx+perPage]
	} else if len(recordInfo) > fromIdx {
		recordInfo = recordInfo[fromIdx:]
	}else{
		recordInfo = []RecordInfo{}
	}
	return 
}

func QuereUserList(db *sql.DB) (rs []map[string]interface{}, err error) {

	var (
		username string
	)
	rows, err := db.Query("select user_name from user")
	defer rows.Close()

	if !checkErr(err) {
		return
	}

	for rows.Next() {
		m := map[string]interface{}{}
		err := rows.Scan(&username)
		m["name"] = username
		rs = append(rs, m)
		checkErr(err)
	}
	return
}

/*
func Update(db *sql.DB) {

	stmt, err := db.Prepare("update user set user_name=?,user_age=? where user_id=?")
	checkErr(err, "prepare err")

	defer stmt.Close()
	if result, err := stmt.Exec("周七", 40, 1); err == nil {
		if c, err := result.RowsAffected(); err == nil {
			fmt.Println("update count : ", c)
		}
	}
}
*/
func Delete(db *sql.DB, userName string) (bl bool, err error) {
	bl, err = CheckIsExist(db, userName)
	if !checkErr(err) {
		return
	}
	if !bl {
		fmt.Printf("%s not exists", userName)
		return
	}

	stmt, err := db.Prepare("delete from user where user_name=?")
	defer stmt.Close()
	if !checkErr(err) {
		return
	}

	if result, err := stmt.Exec(userName); !checkErr(err) {
		return false, err
	} else {
		if c, _ := result.RowsAffected(); err == nil {
			fmt.Println("remove count : ", c)
		}
	}
	return true, nil
}

func Close(db *sql.DB) (err error) {

	err = db.Close()
	if !checkErr(err) {
		return
	}
	return nil
}

func checkErr(err error) bool {
	if err != nil {
		fmt.Printf("checkErr: %s\n", err.Error())
		return false
	}
	return true
}
