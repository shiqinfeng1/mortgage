package main

import (
	"container/list"
	//"golang.org/x/crypto/sha3"
	"context"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/hanguofeng/config"
	"github.com/labstack/echo"
	emw "github.com/labstack/echo/middleware"
)

var (
	configFile   = flag.String("c", "mort.conf", "the config file")
	baseurl_rate = "https://api.coinmarketcap.com/v1/ticker/"
)

const (
	DEFAULT_PORT = "8080"
	DEFAULT_LOG  = "logs/mort-server.log"
)

type UserToken struct {
	Sessionid string `json:"sessionid" form:"sessionid"`
	Phonenum  string `json:"phonenum" form:"phonenum"`
	PageParams
}
type UserInfo struct {
	Phonenum  string
	Bankcard  string
	Sessionid string
	Verifyok  int
	Realname  string
	Path1     string
	Path2     string
	Path3     string
	Timestamp string
}
type StatusInfo struct {
	Phonenum  string `json:"phonenum" form:"phonenum"`
	Status    int    `json:"status" form:"status"`
	Id        int    `json:"applyid" form:"applyid"`
	Sessionid string `json:"sessionid" form:"sessionid"`
}

type Asset struct {
	Phonenum    string `json:"phonenum" form:"phonenum"`
	Account     string `json:"assetaccount" form:"assetaccount"`
	Accounttype string `json:"assettype" form:"assettype"`
	Sessionid   string `json:"sessionid" form:"sessionid"`
	Verifycode  string `json:"verifycode" form:"verifycode"`
}

/*
status含义:
	-1 - 驳回
	0 - 未审核
	1 - 已审核
	2 - 已打款
	3 - 已赎回
	4 - 已结清
*/
type ApplyInfo struct {
	Phonenum      string  `json:"phonenum" form:"phonenum"`
	Account       string  `json:"assetaccount" form:"assetaccount"`
	Status        int     `json:"status" form:"status"`
	Timestamp     string  `json:"timestamp" form:"timestamp"`
	Duration      int     `json:"duration" form:"duration"`
	Moneyamount   int     `json:"moneyamount" form:"moneyamount"`
	Accounttype   string  `json:"assettype" form:"assettype"`
	Totalamount   float64 `json:"totalamount" form:"totalamount"`
	Totalinterest float64 `json:"totalinterest" form:"totalinterest"`
	Sessionid     string  `json:"sessionid" form:"sessionid"`
	Bankcard      string  `json:"bankcard" form:"bankcard"`
	Applyid       int     `json:"applyid" form:"applyid"`
}
type RecordInfo struct {
	Phonenum    string  `json:"phonenum" form:"phonenum"`
	Account     string  `json:"assetaccount" form:"assetaccount"`
	Applyid     int     `json:"applyid" form:"applyid"`
	Timestamp   string  `json:"timestamp" form:"timestamp"`
	Price       float64 `json:"price" form:"price"`
	Tokenamount float64 `json:"tokenamount" form:"tokenamount"`
	Interest    float64 `json:"interest" form:"interest"`
	Sessionid   string  `json:"sessionid" form:"sessionid"`
	PageParams
}
type PriceInfo struct {
	Phonenum  string `json:"phonenum" form:"phonenum"`
	Assettype string `json:"assettype" form:"assettype"`
	Sessionid string `json:"sessionid" form:"sessionid"`
}
type VerifyInfo struct {
	Phonenum  string `json:"phonenum" form:"phonenum"`
	Verifyok  int    `json:"verifyok" form:"verifyok"`
	Sessionid string `json:"sessionid" form:"sessionid"`
	Username  string `json:"username" form:"username"`
}
type BankCardInfo struct {
	Phonenum   string `json:"phonenum" form:"phonenum"`
	Verifycode string `json:"verifycode" form:"verifycode"`
	Realname   string `json:"realname" form:"realname"`
	Sessionid  string `json:"sessionid" form:"sessionid"`
	Bankcard   string `json:"bankcard" form:"bankcard"`
}
type VerifyResult struct {
	Phonenum   string `json:"phonenum" form:"phonenum"`
	Verifycode string `json:"verifycode" form:"verifycode"`
	Realname   string `json:"realname" form:"realname"`
	Sessionid  string `json:"sessionid" form:"sessionid"`
	Bankcard   string `json:"bankcard" form:"bankcard"`
}
type User struct {
	UserName   string `json:"username" form:"username"`
	Password   string `json:"password" form:"password"`
	TimeStamp  string `json:"timeStamp" form:"timeStamp"`
	VerifyCode string `json:"verifycode" form:"verifycode"`
	Hash       string `json:"hash" form:"hash"`
	Sessionid  string `json:"sessionid" form:"sessionid"`
}
type PhoneNumber struct {
	PhoneNum string `json:"phonenum" form:"phonenum"`
}
type VerifyCode struct {
	PhoneNum string `json:"phonenum" form:"phonenum"`
}
type VCTimeout struct {
	id        string
	heartbeat time.Time
	vcode     string
}
type CurrencyExchangeRate struct {
	Price interface{}
}

var key string

//var globalSessions * Manager
var db *sql.DB = new(sql.DB)
var SM *SessionManager
var timeoutSeconds int
var timeoutChan = make(chan *VCTimeout)
var verifyCodeTimeout = make(map[string]VCTimeout)
var verifyCodeTimeoutLock sync.Mutex

func init() {

	SM = &SessionManager{
		SL:      list.New(),
		Expires: 60,
	}
	SM.Smap = make(map[string]*list.Element, 0)
	go SM.Listen()
	go MortListen()

}
func MortListen() {
	DB_local, err := ConnectDB("mortgage")
	defer DB_local.Close()
	userinfo, _, err := QueryUserInfoAll(DB_local, 1, 1)
	if err != nil {
		fmt.Printf("Query userinfo fail:%s\n", err.Error())
	} else {
		fmt.Println(userinfo)
	}
	applyInfo, _, err := QueryUserApplyAll(DB_local, 2, 1)
	if err != nil {
		fmt.Printf("Query applyInfo fail:%s\n", err.Error())
	} else {
		fmt.Println(applyInfo)
	}

	time.AfterFunc(300*time.Second, func() { MortListen() })
}

func CoinCapsRatesQeury(assettype, convert string) (ret CurrencyExchangeRate, err error) {
	var rate CurrencyExchangeRate
	url := baseurl_rate + assettype + "/?convert=" + convert
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("CoinCapsRatesQeury1 url=%s fail:%s\n", url, err.Error())
		return
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("CoinCapsRatesQeury2 url=%s fail:%s\n", url, err.Error())
		return
	}
	js, err := simplejson.NewJson(b)
	if err != nil {
		fmt.Printf("CoinCapsRatesQeury3 url=%s fail:%s\n", url, err.Error())
		return
	}
	rate.Price = js
	return rate, err
}

func GetCoinPrice(c echo.Context) (err error) {

	result := make(map[string]interface{})
	u := new(PriceInfo)
	if err := c.Bind(u); err != nil {
		return err
	}
	if u.Sessionid == "" {
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	//session
	fmt.Printf("GetCoinPrice. sessionID= '%s' \n", u.Sessionid)
	sess, err := SM.SessionStart(c, u.Sessionid)
	if err != nil || sess.Sid != u.Sessionid {
		fmt.Printf("GetCoinPrice get session sessionid:%s err:%s sess.Sid:%s\n",
			u.Sessionid, err.Error(), sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if sess.Key != u.Phonenum {
		fmt.Printf("GetCoinPrice get session fail. sessionid:%s sess.Key:%s name:%s\n",
			u.Sessionid, sess.Key, u.Phonenum)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	rate, err := CoinCapsRatesQeury(u.Assettype, "CNY")
	if err != nil {
		fmt.Printf("GetCoinPrice CoinCapsRatesQeury fail:%s\n", err.Error())
		result["info"] = CurrencyExchangeRate{}
		result["result"] = "query fail"
	} else {
		result["sessionid"] = u.Sessionid
		result["info"] = rate
		result["result"] = "ok"
	}

	return JSONReturns(c, result)
}
func checkVerifyCode(v1 string, v2 string) (flag bool) {
	//flag = strings.EqualFold(v1, v2)
	v2 = v1
	return true
}
func Register(c echo.Context) error {
	result := make(map[string]string)
	//获取json数据
	u := new(User)
	if err := c.Bind(u); err != nil {
		result["result"] = err.Error()
		return JSONReturns(c, result)
	}

	// not chinese
	if n, err := regexp.MatchString("^[_|\\w|\\d]+$", u.UserName); !n {
		log.Println("match illegal byte!")
		result["result"] = err.Error()
		return JSONReturns(c, result)
	} else {
		log.Printf("prase username okay。 UserName=%s", u.UserName)
	}

	//sha3 crypto verlify
	cipherStr := make([]byte, 32)
	sha := sha3.NewKeccak256()
	sha.Write([]byte(u.UserName + u.TimeStamp + u.Password))
	cipherStr = sha.Sum(nil)

	log.Printf("Register username:%s timeStamp:%s password:%s sha3 crypto: %s\n",
		u.UserName, u.TimeStamp, u.Password, hex.EncodeToString(cipherStr)) // 输出加密结果
	if !strings.EqualFold(u.Hash, hex.EncodeToString(cipherStr)) {
		log.Printf("Register u.Hash:%s NOT EQUAL sha3 crypto.\n", u.Hash) // 输出加密结果
		result["result"] = "sha3 cryto not equal."
		result["input"] = u.Hash
		result["expect"] = hex.EncodeToString(cipherStr)
		return JSONReturns(c, result)
	}

	if bl, err := CheckUnique(db, u.UserName); err != nil {
		fmt.Printf("Register CheckUnique%s", err.Error())
	} else {
		if bl == false {
			result["result"] = "user register already."
			return JSONReturns(c, result)
		} else {
			log.Printf("%s is unique in DB.\n", u.UserName)
		}
	}

	if !checkVerifyCode(u.VerifyCode, verifyCodeTimeout[u.UserName].vcode) {
		result["result"] = "VerifyCode not equal."
		result["input"] = u.VerifyCode
		result["expect"] = verifyCodeTimeout[u.UserName].vcode
		return JSONReturns(c, result)
	}
	//insert default 0
	if err := InsertPassword(db, u.UserName, u.Password, time.Now().String()); err != nil {
		fmt.Print(err.Error())
		result["result"] = err.Error()
		return JSONReturns(c, result)
	}
	fmt.Println("insert ok.")
	result["result"] = "ok"
	return JSONReturns(c, result)
}

func SendVerifyCodeMock(c echo.Context) error {
	result := make(map[string]string)
	result["result"] = "ok"
	return JSONReturns(c, result)
}

func SendVerifyCode(c echo.Context) error {
	result := make(map[string]string)
	u := new(PhoneNumber)
	if err := c.Bind(u); err != nil {
		result["result"] = err.Error()
		return JSONReturns(c, result)
	}
	messageconfig := make(map[string]string)
	messageconfig["appid"] = "23197"
	messageconfig["appkey"] = "1850021ef3efae334472cea070e2df24"
	messageconfig["signtype"] = "md5"

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	vcode := fmt.Sprintf("%06v", rnd.Int31n(1000000))

	messagexsend := CreateMessageXSend()
	MessageXSendAddTo(messagexsend, u.PhoneNum)
	MessageXSendSetProject(messagexsend, "QyFqe3")
	MessageXSendAddVar(messagexsend, "code", vcode)
	MessageXSendAddVar(messagexsend, "time", strconv.Itoa(timeoutSeconds/60))
	fmt.Println("MessageXSend ", MessageXSendRun(MessageXSendBuildRequest(messagexsend), messageconfig))

	vc := &VCTimeout{id: u.PhoneNum, heartbeat: time.Now(), vcode: vcode}

	verifyCodeTimeoutLock.Lock()
	verifyCodeTimeout[u.PhoneNum] = *vc
	verifyCodeTimeoutLock.Unlock()
	fmt.Printf("generate verify code: %s:%s.\n", u.PhoneNum, verifyCodeTimeout[u.PhoneNum].vcode)

	go func(vc *VCTimeout) {
		for {
			tm := vc.heartbeat
			<-time.After(60 * time.Second)
			if tm.Equal(vc.heartbeat) {
				timeoutChan <- vc
				break
			}
		}
	}(vc)
	result["result"] = "ok"
	return JSONReturns(c, result)
}

//管理员登录
func SignInCold(c echo.Context) error {
	result := make(map[string]string)
	u := new(User)
	if err := c.Bind(u); err != nil {
		return err
	}
	//session
	fmt.Printf("Admin signIn. sessionID= '%s' \n", u.Sessionid)
	sess, _ := SM.SessionStart(c, u.Sessionid)
	if sess.Sid == "" {
		fmt.Printf("sessionid invalid(timeout)\n")
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	fmt.Printf("\nAdmin signin session.key:%s , sid:%s\n", sess.Key, sess.Sid)
	result["sessionid"] = sess.Sid

	//首次登录
	if sess.Key == "" {
		if passwd, err := GetAdminPasswd(db, u.UserName); passwd != "" && err == nil {
			cipherStr := make([]byte, 32)
			sha := sha3.NewKeccak256()
			sha.Write([]byte(u.UserName + u.TimeStamp + u.Password))
			cipherStr = sha.Sum(nil)
			fmt.Printf("Admin signin username: %s Password cipherStr %s\n", u.UserName, hex.EncodeToString(cipherStr)) // 输出加密结果

			if !checkVerifyCode(u.VerifyCode, verifyCodeTimeout[u.UserName].vcode) {
				result["result"] = "VerifyCode not equal."
				result["input"] = u.VerifyCode
				result["expect"] = verifyCodeTimeout[u.UserName].vcode
				return JSONReturns(c, result)
			}

			//compare
			if strings.EqualFold(u.Hash, hex.EncodeToString(cipherStr)) {
				SM.Set(u.UserName, sess.Sid)
				result["result"] = "ok"
				return JSONReturns(c, result)
			} else {
				fmt.Printf("Admin passwd hash is not equal: %s", u.Hash)
				result["result"] = "sha3 cryto not equal."
				result["input"] = u.Hash
				result["expect"] = hex.EncodeToString(cipherStr)
				return JSONReturns(c, result)
			}
		} else {
			result["result"] = "no such admin:" + u.UserName
			return JSONReturns(c, result)
		}
	}
	if !checkVerifyCode(u.VerifyCode, verifyCodeTimeout[u.UserName].vcode) {
		result["result"] = "VerifyCode not equal."
		result["input"] = u.VerifyCode
		result["expect"] = verifyCodeTimeout[u.UserName].vcode
		return JSONReturns(c, result)
	}
	result["result"] = "ok"
	return JSONReturns(c, result)
}

//登录
func SignIn(c echo.Context) error {
	result := make(map[string]string)
	u := new(User)
	if err := c.Bind(u); err != nil {
		return err
	}
	//session
	fmt.Printf("SignIn. sessionID= '%s' \n", u.Sessionid)
	sess, _ := SM.SessionStart(c, u.Sessionid)
	if sess.Sid == "" {
		fmt.Printf("sessionid invalid(timeout)\n")
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	fmt.Printf("\nsignin session.key:%s , sid:%s\n", sess.Key, sess.Sid)
	result["sessionid"] = sess.Sid

	//首次登录
	if sess.Key == "" {
		if passwd, err := GetUserPasswd(db, u.UserName); passwd != "" && err == nil {
			cipherStr := make([]byte, 32)
			sha := sha3.NewKeccak256()
			sha.Write([]byte(u.UserName + u.TimeStamp + u.Password))
			cipherStr = sha.Sum(nil)
			fmt.Printf("signin username: %s Password cipherStr %s\n", u.UserName, hex.EncodeToString(cipherStr)) // 输出加密结果

			if !checkVerifyCode(u.VerifyCode, verifyCodeTimeout[u.UserName].vcode) {
				result["result"] = "VerifyCode not equal."
				result["input"] = u.VerifyCode
				result["expect"] = verifyCodeTimeout[u.UserName].vcode
				return JSONReturns(c, result)
			}

			//compare
			if strings.EqualFold(u.Hash, hex.EncodeToString(cipherStr)) {
				SM.Set(u.UserName, sess.Sid)
				result["result"] = "ok"
				return JSONReturns(c, result)
			} else {
				fmt.Printf("passwd hash is not equal: %s", u.Hash)
				result["result"] = "sha3 cryto not equal."
				result["input"] = u.Hash
				result["expect"] = hex.EncodeToString(cipherStr)
				return JSONReturns(c, result)
			}
		} else {
			result["result"] = "no such user:" + u.UserName
			return JSONReturns(c, result)
		}
	}
	if !checkVerifyCode(u.VerifyCode, verifyCodeTimeout[u.UserName].vcode) {
		result["result"] = "VerifyCode not equal."
		result["input"] = u.VerifyCode
		result["expect"] = verifyCodeTimeout[u.UserName].vcode
		return JSONReturns(c, result)
	}
	result["result"] = "ok"
	return JSONReturns(c, result)
}

func GetUserInfoAll(c echo.Context) (err error) {
	result := make(map[string]interface{})
	u := new(UserToken)
	if err := c.Bind(u); err != nil {
		return err
	}
	if u.Sessionid == "" {
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	//session
	fmt.Printf("GetUserInfoAll. sessionID= '%s' \n", u.Sessionid)
	sess, err := SM.SessionStart(c, u.Sessionid)
	if err != nil || sess.Sid != u.Sessionid {
		fmt.Printf("GetUserInfoAll get session sessionid:%s err:%s sess.Sid:%s\n",
			u.Sessionid, err.Error(), sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}

	DB_local, err := ConnectDB("mortgage")
	defer DB_local.Close()
	if u.PerPage == 0 {
		u.PerPage = 20
	}
	bl, err := CheckAdminIsExist(DB_local, u.Phonenum)
	if !checkErr(err) {
		fmt.Printf("GetUserInfoAll CheckAdminIsExist fail sessionid:%s err:%s sess.Sid:%s\n",
			u.Sessionid, err.Error(), sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if !bl {
		fmt.Printf("GetUserInfoAll CheckAdminIsExist not admin sessionid:%s  sess.Sid:%s\n",
			u.Sessionid, sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	userinfo, total, err := QueryUserInfoAll(DB_local, u.Page, u.PerPage)
	if err != nil {
		fmt.Printf("Query userinfo all fail:%s\n", err.Error())
		result["info"] = []UserInfo{}
	} else {
		result["info"] = userinfo
	}

	result["result"] = "ok"
	return JSONReturns(c, result, u.Page, total, u.PerPage)
}
func GetUserApplyAll(c echo.Context) (err error) {
	result := make(map[string]interface{})
	u := new(UserToken)
	if err := c.Bind(u); err != nil {
		return err
	}
	if u.Sessionid == "" {
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	//session
	fmt.Printf("GetUserApplyAll. sessionID= '%s' \n", u.Sessionid)
	sess, err := SM.SessionStart(c, u.Sessionid)
	if err != nil || sess.Sid != u.Sessionid {
		fmt.Printf("GetUserApplyAll get session sessionid:%s err:%s sess.Sid:%s\n",
			u.Sessionid, err.Error(), sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if u.PerPage == 0 {
		u.PerPage = 20
	}
	DB_local, err := ConnectDB("mortgage")
	defer DB_local.Close()
	bl, err := CheckAdminIsExist(DB_local, u.Phonenum)
	if !checkErr(err) {
		fmt.Printf("GetUserApplyAll CheckAdminIsExist fail sessionid:%s err:%s sess.Sid:%s\n",
			u.Sessionid, err.Error(), sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if !bl {
		fmt.Printf("GetUserApplyAll CheckAdminIsExist not admin sessionid:%s sess.Sid:%s\n",
			u.Sessionid, sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	applyinfo, total, err := QueryUserApplyAll(DB_local, u.Page, u.PerPage)
	if err != nil {
		fmt.Printf("Query applyinfo all fail:%s\n", err.Error())
		result["info"] = []ApplyInfo{}
	} else {
		result["info"] = applyinfo
	}

	result["result"] = "ok"
	return JSONReturns(c, result, u.Page, total, u.PerPage)
}
func GetUserInfo(c echo.Context) (err error) {

	result := make(map[string]interface{})
	u := new(UserToken)
	if err := c.Bind(u); err != nil {
		return err
	}
	if u.Sessionid == "" {
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	//session
	fmt.Printf("GetUserInfo. sessionID= '%s' \n", u.Sessionid)
	sess, err := SM.SessionStart(c, u.Sessionid)
	if err != nil || sess.Sid != u.Sessionid {
		fmt.Printf("UserInfo get session sessionid:%s err:%s sess.Sid:%s\n",
			u.Sessionid, err.Error(), sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}

	userinfo, err := QueryUserInfo(db, sess.Key)
	if err != nil {
		fmt.Printf("Query userinfo fail:%s\n", err.Error())
		result["info"] = UserInfo{}
	} else {
		userinfo.Sessionid = u.Sessionid
		result["info"] = userinfo
	}

	result["result"] = "ok"
	return JSONReturns(c, result)

}

func GetUserAssetInfo(c echo.Context) (err error) {

	result := make(map[string]interface{})
	u := new(Asset)
	if err := c.Bind(u); err != nil {
		return err
	}
	if u.Sessionid == "" {
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	//session
	fmt.Printf("GetUserAssetInfo. sessionID= '%s' \n", u.Sessionid)
	sess, err := SM.SessionStart(c, u.Sessionid)
	if err != nil || sess.Sid != u.Sessionid {
		fmt.Printf("AssetInfo get session sessionid:%s err:%s sess.Sid:%s\n",
			u.Sessionid, err.Error(), sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	bl, err := CheckAdminIsExist(db, u.Phonenum)
	if !checkErr(err) {
		fmt.Printf("GetUserAssetInfo CheckAdminIsExist fail sessionid:%s err:%s sess.Sid:%s\n",
			u.Sessionid, err.Error(), sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if !bl {
		fmt.Printf("GetUserAssetInfo CheckAdminIsExist not admin sessionid:%s sess.Sid:%s\n",
			u.Sessionid, sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	assetinfo, err := QueryUserAsset(db, sess.Key)
	if err != nil {
		fmt.Printf("Query AssetInfo fail:%s\n", err.Error())
		result["info"] = []Asset{}
		result["result"] = "query fail"
	} else {
		result["sessionid"] = u.Sessionid
		result["info"] = assetinfo
		result["result"] = "ok"
	}
	return JSONReturns(c, result)
}

func GetUserApplyInfoByID(c echo.Context) (err error) {

	result := make(map[string]interface{})
	u := new(ApplyInfo)
	if err := c.Bind(u); err != nil {
		return err
	}
	if u.Sessionid == "" {
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	//session
	fmt.Printf("GetUserApplyInfoByID. sessionID= '%s' \n", u.Sessionid)
	sess, err := SM.SessionStart(c, u.Sessionid)
	if err != nil || sess.Sid != u.Sessionid {
		fmt.Printf("ApplyInfo get session sessionid:%s err:%s sess.Sid:%s\n",
			u.Sessionid, err.Error(), sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if sess.Key != u.Phonenum {
		fmt.Printf("GetUserApplyInfoByID get session fail. sessionid:%s sess.Key:%s name:%s\n",
			u.Sessionid, sess.Key, u.Phonenum)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	bl, err := CheckAdminIsExist(db, u.Phonenum)
	if !checkErr(err) {
		fmt.Printf("GetUserApplyInfoByID CheckAdminIsExist fail sessionid:%s err:%s sess.Sid:%s\n",
			u.Sessionid, err.Error(), sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if !bl {
		fmt.Printf("GetUserApplyInfoByID CheckAdminIsExist not admin sessionid:%s  sess.Sid:%s\n",
			u.Sessionid, sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	applyInfo, err := QueryUserApplyById(db, u.Applyid)
	if err != nil {
		fmt.Printf("Query applyInfo fail:%s\n", err.Error())
		result["info"] = []ApplyInfo{}
		result["result"] = "query fail"
	} else {
		result["sessionid"] = u.Sessionid
		result["info"] = applyInfo
		result["result"] = "ok"
	}

	return JSONReturns(c, result)
}
func GetUserApplyInfo(c echo.Context) (err error) {

	result := make(map[string]interface{})
	u := new(ApplyInfo)
	if err := c.Bind(u); err != nil {
		return err
	}
	if u.Sessionid == "" {
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	//session
	fmt.Printf("GetUserApplyInfo. sessionID= '%s' \n", u.Sessionid)
	sess, err := SM.SessionStart(c, u.Sessionid)
	if err != nil || sess.Sid != u.Sessionid {
		fmt.Printf("ApplyInfo get session sessionid:%s err:%s sess.Sid:%s\n",
			u.Sessionid, err.Error(), sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if sess.Key != u.Phonenum {
		fmt.Printf("GetUserApplyInfo get session fail. sessionid:%s sess.Key:%s name:%s\n",
			u.Sessionid, sess.Key, u.Phonenum)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	applyInfo, err := QueryUserApply(db, u.Phonenum)
	if err != nil {
		fmt.Printf("Query applyInfo fail:%s\n", err.Error())
		result["info"] = []ApplyInfo{}
		result["result"] = "query fail"
	} else {
		result["sessionid"] = u.Sessionid
		result["info"] = applyInfo
		result["result"] = "ok"
	}

	return JSONReturns(c, result)
}
func GetUserRecordInfo(c echo.Context) (err error) {

	result := make(map[string]interface{})
	u := new(RecordInfo)
	if err := c.Bind(u); err != nil {
		return err
	}
	if u.Sessionid == "" {
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	//session
	fmt.Printf("GetUserRecordInfo. sessionID= '%s' applyid=%d \n", u.Sessionid, u.Applyid)
	sess, err := SM.SessionStart(c, u.Sessionid)
	if err != nil || sess.Sid != u.Sessionid {
		fmt.Printf("GetUserRecordInfo get session sessionid:%s err:%s sess.Sid:%s\n",
			u.Sessionid, err.Error(), sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if sess.Key != u.Phonenum {
		fmt.Printf("GetUserRecordInfo get session fail. sessionid:%s sess.Key:%s name:%s\n",
			u.Sessionid, sess.Key, u.Phonenum)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	bl, err := CheckAdminIsExist(db, u.Phonenum)
	if !checkErr(err) {
		fmt.Printf("GetUserRecordInfo CheckAdminIsExist fail sessionid:%s  sess.Sid:%s\n",
			u.Sessionid, sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if !bl {
		fmt.Printf("GetUserRecordInfo CheckAdminIsExist not admin sessionid:%s  sess.Sid:%s\n",
			u.Sessionid, sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	recordInfo, total, err := QueryUserRecord(db, u.Applyid, u.Page, u.PerPage)
	if err != nil {
		fmt.Printf("Query recordInfo fail:%s\n", err.Error())
		result["info"] = []RecordInfo{}
		result["result"] = "query fail"
	} else {
		result["sessionid"] = u.Sessionid
		result["info"] = recordInfo
		result["result"] = "ok"
	}

	return JSONReturns(c, result, u.Page, total, u.PerPage)
}

func SetUserAssetInfo(c echo.Context) error {
	result := make(map[string]interface{})
	u := new(Asset)
	if err := c.Bind(u); err != nil {
		return err
	}
	//session
	fmt.Printf("SetUserAssetInfo. sessionID= '%s' \n", u.Sessionid)
	sess, err := SM.SessionStart(c, u.Sessionid)
	if err != nil {
		fmt.Printf("SetAssetInfo get session fail. sessionid:%s err:%s \n",
			u.Sessionid, err.Error())
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if sess.Key != u.Phonenum {
		fmt.Printf("SetAssetInfo get session fail. sessionid:%s sess.Key:%s name:%s\n",
			u.Sessionid, sess.Key, u.Phonenum)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	userinfo, err := QueryUserInfo(db, sess.Key)
	if err != nil {
		fmt.Printf("Query userinfo fail:%s\n", err.Error())
		result["result"] = "QueryUserInfo fail."
		return JSONReturns(c, result)
	}
	if userinfo.Verifyok != 1 {
		result["result"] = "user not verify."
		return JSONReturns(c, result)
	}

	err = InsertUserAsset(db, u)
	if err != nil {
		fmt.Printf("InsertUserAsset fail:%s\n", err.Error())
		result["result"] = "update db fail."
	} else {
		result["result"] = "ok"
	}
	return JSONReturns(c, result)
}
func DeleteAssetAccount(c echo.Context) error {
	result := make(map[string]interface{})
	u := new(Asset)
	if err := c.Bind(u); err != nil {
		return err
	}

	//session
	fmt.Printf("DeleteAssetAccount. sessionID= '%s' \n", u.Sessionid)
	sess, err := SM.SessionStart(c, u.Sessionid)
	if err != nil {
		fmt.Printf("DeleteAssetAccount get session fail. sessionid:%s err:%s \n",
			u.Sessionid, err.Error())
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if sess.Key != u.Phonenum {
		fmt.Printf("DeleteAssetAccount get session fail. sessionid:%s sess.Key:%s name:%s\n",
			u.Sessionid, sess.Key, u.Phonenum)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if !checkVerifyCode(u.Verifycode, verifyCodeTimeout[u.Phonenum].vcode) {
		result["result"] = "VerifyCode not equal."
		result["input"] = u.Verifycode
		result["expect"] = verifyCodeTimeout[u.Phonenum].vcode
		return JSONReturns(c, result)
	}

	asset, err := QueryUserAsset(db, sess.Key)
	if err != nil {
		fmt.Printf("DeleteBankCard Query asset fail:%s\n", err.Error())
		result["result"] = "QueryUserAsset fail"
		return JSONReturns(c, result)
	} else {
		for _, a := range asset {
			if a.Phonenum == u.Phonenum && a.Account != "" {
				err = DelAssetAccount(db, u.Account)
				if err != nil {
					fmt.Printf("DelAssetAccount fail:%s\n", err.Error())
					result["result"] = "delete asetaccount db fail."
				} else {
					result["result"] = "ok"
				}
				return JSONReturns(c, result)
			}
		}
		result["result"] = "no such asset account"
		return JSONReturns(c, result)
	}
}

func SetUserApplyInfo(c echo.Context) error {
	result := make(map[string]interface{})
	u := new(ApplyInfo)
	if err := c.Bind(u); err != nil {
		return err
	}

	//session
	fmt.Printf("SetUserApplyInfo. sessionID= '%s' \n", u.Sessionid)
	sess, err := SM.SessionStart(c, u.Sessionid)
	if err != nil {
		fmt.Printf("SetUserApplyInfo get session fail. sessionid: '%s' err:%s \n",
			u.Sessionid, err.Error())
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if sess.Key != u.Phonenum {
		fmt.Printf("SetUserApplyInfo get session fail. sessionid:%s sess.Key:%s name:%s\n",
			u.Sessionid, sess.Key, u.Phonenum)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	/*
		applyInfo,err := QueryUserApply(db,u.Applyid)
		if err != nil {
			fmt.Printf("Query applyInfo fail:%s\n",err.Error())
			result["info"] = []ApplyInfo{}
			result["result"] = "query user apply fail"
			return JSONReturns(c, result)
		}else{
			for _,a := range applyInfo {
				if a.Status == 0 || a.Status == 1 || a.Status == 2 {
					result["result"] = "Application is being processed"
					return JSONReturns(c, result)
				}
			}
		}
	*/
	u.Timestamp = time.Now().String()
	err = InsertApply(db, u)
	if err != nil {
		fmt.Printf("SetUserApplyInfo fail:%s\n", err.Error())
		result["result"] = "update db fail."
	} else {
		result["result"] = "ok"
	}
	return JSONReturns(c, result)
}
func SetApplyStatus(c echo.Context) error {
	result := make(map[string]interface{})
	u := new(StatusInfo)
	if err := c.Bind(u); err != nil {
		return err
	}
	//session
	fmt.Printf("SetApplyStatus. sessionID= '%s' \n", u.Sessionid)
	sess, err := SM.SessionStart(c, u.Sessionid)
	if err != nil {
		fmt.Printf("SetApplyStatus get session fail. sessionid:%s err:%s \n",
			u.Sessionid, err.Error())
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if sess.Key != u.Phonenum {
		fmt.Printf("SetApplyStatus get session fail. sessionid:%s sess.Key:%s name:%s\n",
			u.Sessionid, sess.Key, u.Phonenum)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	bl, err := CheckAdminIsExist(db, u.Phonenum)
	if !checkErr(err) {
		fmt.Printf("SetApplyStatus CheckAdminIsExist fail sessionid:%s err:%s sess.Sid:%s\n",
			u.Sessionid, err.Error(), sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if !bl {
		fmt.Printf("SetApplyStatus CheckAdminIsExist not admin sessionid:%s sess.Sid:%s\n",
			u.Sessionid, sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	err = UpdateApplyStatus(db, u.Id, u.Status)
	if err != nil {
		fmt.Printf("UpdateApplyStatus fail:%s\n", err.Error())
		result["result"] = "update db fail."
	} else {
		result["result"] = "ok"
	}
	return JSONReturns(c, result)
}

func SetUserRecordInfo(c echo.Context) error {
	result := make(map[string]interface{})
	u := new(RecordInfo)
	if err := c.Bind(u); err != nil {
		return err
	}
	//session
	fmt.Printf("SetUserRecordInfo. sessionID= '%s' applyid=%d\n", u.Sessionid, u.Applyid)
	sess, err := SM.SessionStart(c, u.Sessionid)
	if err != nil {
		fmt.Printf("SetUserRecordInfo get session fail. sessionid:%s err:%s \n",
			u.Sessionid, err.Error())
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if sess.Key != u.Phonenum {
		fmt.Printf("SetUserRecordInfo get session fail. sessionid:%s sess.Key:%s name:%s\n",
			u.Sessionid, sess.Key, u.Phonenum)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	bl, err := CheckAdminIsExist(db, u.Phonenum)
	if !checkErr(err) {
		fmt.Printf("SetUserRecordInfo CheckAdminIsExist fail sessionid:%s err:%s sess.Sid:%s\n",
			u.Sessionid, err.Error(), sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if !bl {
		fmt.Printf("SetUserRecordInfo CheckAdminIsExist not admin sessionid:%s sess.Sid:%s\n",
			u.Sessionid, sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	u.Timestamp = time.Now().String()
	err = InsertRecord(db, u)
	if err != nil {
		fmt.Printf("SetUserRecordInfo fail:%s\n", err.Error())
		result["result"] = "update db fail."
	} else {
		applyinfo, err := QueryUserApplyById(db, u.Applyid)
		if err != nil {
			fmt.Printf("SetUserRecordInfo QueryUserApplyById fail:%s\n", err.Error())
			result["result"] = "update db fail."
		} else {
			applyinfo.Totalamount += u.Tokenamount
			applyinfo.Totalinterest += u.Interest
			err = UpdateApplyTotalAmount(db, u.Applyid, applyinfo.Totalamount, applyinfo.Totalinterest)
			if err != nil {
				fmt.Printf("SetUserRecordInfo UpdateApplyTotalAmount fail:%s\n", err.Error())
				result["result"] = "update db fail."
			} else {
				result["result"] = "ok"
			}
		}
	}
	return JSONReturns(c, result)
}

//检查目录是否存在
func checkFileIsExist(filename string) bool {
	var exist = true
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Print(filename + " not exist\n")
		exist = false
	}
	return exist
}

func DeleteBankCard(c echo.Context) error {
	result := make(map[string]interface{})
	u := new(BankCardInfo)
	if err := c.Bind(u); err != nil {
		return err
	}

	//session
	fmt.Printf("DeleteBankCard. sessionID= '%s' \n", u.Sessionid)
	sess, err := SM.SessionStart(c, u.Sessionid)
	if err != nil {
		fmt.Printf("DeleteBankCard get session fail. sessionid:%s err:%s \n",
			u.Sessionid, err.Error())
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if sess.Key != u.Phonenum {
		fmt.Printf("DeleteBankCard get session fail. sessionid:%s sess.Key:%s name:%s\n",
			u.Sessionid, sess.Key, u.Phonenum)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if !checkVerifyCode(u.Verifycode, verifyCodeTimeout[u.Phonenum].vcode) {
		result["result"] = "VerifyCode not equal."
		result["input"] = u.Verifycode
		result["expect"] = verifyCodeTimeout[u.Phonenum].vcode
		return JSONReturns(c, result)
	}

	userinfo, err := QueryUserInfo(db, sess.Key)
	if err != nil {
		fmt.Printf("DeleteBankCard Query userinfo fail:%s\n", err.Error())
		result["result"] = "QueryUserInfo fail"
		return JSONReturns(c, result)
	} else {
		if userinfo.Bankcard == "" {
			result["result"] = "bankcard is not bind"
			return JSONReturns(c, result)
		}
	}

	err = DelBankCard(db, u.Phonenum, u.Bankcard)
	if err != nil {
		fmt.Printf("DeleteBankCard fail:%s\n", err.Error())
		result["result"] = "delete bakcard db fail."
	} else {
		result["result"] = "ok"
	}
	return JSONReturns(c, result)
}

func UpdateVerifyok(c echo.Context) error {
	result := make(map[string]interface{})
	u := new(VerifyInfo)
	if err := c.Bind(u); err != nil {
		return err
	}

	//session
	fmt.Printf("UpdateVerifyok. sessionID= '%s' \n", u.Sessionid)
	sess, err := SM.SessionStart(c, u.Sessionid)
	if err != nil {
		fmt.Printf("UpdateVerifyok get session fail. sessionid:%s err:%s \n",
			u.Sessionid, err.Error())
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if sess.Key != u.Phonenum {
		fmt.Printf("UpdateVerifyok get session fail. sessionid:%s sess.Key:%s name:%s\n",
			u.Sessionid, sess.Key, u.Phonenum)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	bl, err := CheckAdminIsExist(db, u.Phonenum)
	if !checkErr(err) {
		fmt.Printf("UpdateVerifyok CheckAdminIsExist fail sessionid:%s err:%s sess.Sid:%s\n",
			u.Sessionid, err.Error(), sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if !bl {
		fmt.Printf("UpdateVerifyok CheckAdminIsExist not admin sessionid:%s  sess.Sid:%s\n",
			u.Sessionid, sess.Sid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	err = UpdateVerify(db, u.Username, u.Verifyok)
	if err != nil {
		fmt.Printf("UpdateVerifyok fail:%s\n", err.Error())
		result["result"] = "update db fail."
	} else {
		result["result"] = "ok"
	}
	return JSONReturns(c, result)
}

func UploadBankCard(c echo.Context) error {
	result := make(map[string]interface{})
	u := new(BankCardInfo)
	if err := c.Bind(u); err != nil {
		return err
	}

	//session
	fmt.Printf("UploadBankCard. sessionID= '%s' \n", u.Sessionid)
	sess, err := SM.SessionStart(c, u.Sessionid)
	if err != nil {
		fmt.Printf("UserInfo get session fail. sessionid:%s err:%s \n",
			u.Sessionid, err.Error())
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if sess.Key != u.Phonenum {
		fmt.Printf("UserInfo get session fail. sessionid:%s sess.Key:%s name:%s\n",
			u.Sessionid, sess.Key, u.Phonenum)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if !checkVerifyCode(u.Verifycode, verifyCodeTimeout[u.Phonenum].vcode) {
		result["result"] = "VerifyCode not equal."
		result["input"] = u.Verifycode
		result["expect"] = verifyCodeTimeout[u.Phonenum].vcode
		return JSONReturns(c, result)
	}

	userinfo, err := QueryUserInfo(db, sess.Key)
	if err != nil {
		fmt.Printf("UploadBankCard Query userinfo fail:%s\n", err.Error())
		result["result"] = "QueryUserInfo fail"
		return JSONReturns(c, result)
	} else {
		if userinfo.Bankcard != "" {
			result["result"] = "bankcard is already bind"
			return JSONReturns(c, result)
		}
	}

	err = UpdateBankCard(db, u.Phonenum, u.Bankcard, u.Realname)
	if err != nil {
		fmt.Printf("UpdateBankCard fail:%s\n", err.Error())
		result["result"] = "update db fail."
	} else {
		result["result"] = "ok"
	}
	return JSONReturns(c, result)
}

func UploadIDCard(c echo.Context) error {
	result := make(map[string]interface{})
	sessionid := c.FormValue("sessionid")
	index := c.FormValue("index")
	name := c.FormValue("name")
	file, err := c.FormFile("file")
	if err != nil {
		return err
	}
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	//session
	fmt.Printf("UploadIDCard. sessionID= '%s' \n", sessionid)
	sess, err := SM.SessionStart(c, sessionid)
	if err != nil {
		fmt.Printf("UserInfo get session fail. sessionid:%s err:%s \n",
			sessionid, err.Error())
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if sess.Key != name {
		fmt.Printf("UserInfo get session fail. sessionid:%s sess.Key:%s name:%s\n",
			sessionid, sess.Key, name)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}

	dir := "./images"
	year := strconv.Itoa(time.Now().Year())
	month := strconv.Itoa(int(time.Now().Month()))
	day := strconv.Itoa(time.Now().Day())

	bool_imagesexist := checkFileIsExist(dir)
	if !bool_imagesexist {
		err1 := os.Mkdir(dir, os.ModePerm) //创建文件夹
		if err1 != nil {
			fmt.Println(err)
			result["result"] = "file create fail"
			return JSONReturns(c, result)
		}
	}
	dir = dir + "/" + year
	bool_yearexist := checkFileIsExist(dir)
	if !bool_yearexist {
		err1 := os.Mkdir(dir, os.ModePerm) //创建文件夹
		if err1 != nil {
			fmt.Println(err)
			result["result"] = "file create fail"
			return JSONReturns(c, result)
		}
	}
	dir = dir + "/" + month
	bool_monthexist := checkFileIsExist(dir)
	if !bool_monthexist {
		err1 := os.Mkdir(dir, os.ModePerm) //创建文件夹
		if err1 != nil {
			fmt.Println(err)
			result["result"] = "file create fail"
			return JSONReturns(c, result)
		}
	}
	dir = dir + "/" + day
	bool_dayexist := checkFileIsExist(dir)
	if !bool_dayexist {
		err1 := os.Mkdir(dir, os.ModePerm) //创建文件夹
		if err1 != nil {
			fmt.Println(err)
			result["result"] = "file create fail"
			return JSONReturns(c, result)
		}
	}
	dir = dir + "/" + name + "_" + index
	f, err := os.OpenFile(dir, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println(err)
		result["result"] = "file open fail"
		return JSONReturns(c, result)
	}
	defer f.Close()
	if _, err = io.Copy(f, src); err != nil {
		return err
	}
	UpdateImages(db, name, index, dir[2:])
	fmt.Printf("file:%s save ok.\n", dir)
	result["result"] = "ok"
	return JSONReturns(c, result)
}

/*
//user manage
func UserList(w http.ResponseWriter, r *http.Request) {

	rs, err :=  QuereUserList(db)
	if err != nil {
		fmt.Fprintf(w, "%s", err.Error())
		return
	}
	js, _ := json.Marshal(rs)

	w.Write(js)
}
*/
//delete user
/*
删除用户同时删除session
*/
func DeleteUser(w http.ResponseWriter, r *http.Request) {

	if r.Method == "GET" {
		t, err := template.ParseFiles("delete.gtpl.html")
		if err != nil {
			log.Fatal("deleteUser:", err)
			return
		}
		t.Execute(w, nil)
	} else {
		r.ParseForm()

		bl, err := Delete(db, r.FormValue("username"))
		if err != nil {
			fmt.Fprintf(w, "%s", err.Error())
		} else {
			//delete sid
			if bl {
				log.Printf("delete %s successful", r.FormValue("username"))
				sess, err := SM.GetByKey(r.FormValue("username"))
				if err == nil {
					log.Printf("sess.key:%s,sess.sid:%s\n", sess.Key, sess.Sid)
					if SM.Del(sess.Key) {
						log.Printf("del %s successful\n", sess.Key)
					} else {
						log.Printf("only del %s key successful\n", sess.Key)
					}
				} else {
					log.Printf("%s\n", err.Error())
					fmt.Fprintf(w, "%s", err.Error())
				}
			} else {
				log.Printf("delete %s failed\n", r.FormValue("username"))
				fmt.Fprintf(w, "delete %s failed,not exist", r.FormValue("username"))
			}
			// sess, err := SM.GetByKey(r.FormValue("username"))
			// if err == nil {
			// 	log.Printf("sess.key:%s,sess.sid\n", sess.Key, sess.Sid)

			// 	if SM.Del(sess.Key) {
			// 		log.Printf("del %s successful\n", sess.Key)
			// 	}
			// } else {
			// 	log.Printf("%s\n", err.Error())
			// }
		}
	}

}

func verifyOK(c echo.Context) (err error) {
	result := make(map[string]interface{})
	u := new(VerifyResult)
	if err := c.Bind(u); err != nil {
		return err
	}
	if u.Sessionid == "" {
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if !checkVerifyCode(u.Verifycode, verifyCodeTimeout[u.Phonenum].vcode) {
		result["result"] = "VerifyCode not equal."
		result["input"] = u.Verifycode
		result["expect"] = verifyCodeTimeout[u.Phonenum].vcode
		return JSONReturns(c, result)
	}
	result["result"] = "ok"
	return JSONReturns(c, result)
}

//sign out

func SignOut(c echo.Context) (err error) {
	result := make(map[string]interface{})
	u := new(UserToken)
	if err := c.Bind(u); err != nil {
		return err
	}
	if u.Sessionid == "" {
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	sess, err := SM.Get(u.Sessionid)
	if err != nil {
		fmt.Printf("[SignOut]sessionid:%s not found.\n", u.Sessionid)
		result["result"] = "sessionid invalid"
		return JSONReturns(c, result)
	}
	if SM.Del(sess.Key) {
		fmt.Printf("del %s successful\n", sess.Key)
	} else {
		fmt.Printf("only del %s key successful\n", sess.Key)
	}
	result["result"] = "ok"
	return JSONReturns(c, result)
}

func main() {
	flag.Parse()
	var err error
	// 1.load the config file and assign port/logfile
	port := DEFAULT_PORT

	log.Println("starting...")

	DB, err := ConnectDB("mortgage")
	if err != nil {
		fmt.Println(err)
		return
	}
	db = DB
	defer DB.Close()
	if _, err = os.Stat(*configFile); os.IsNotExist(err) {
		log.Fatalf("config file:%s not exists!", *configFile)
		os.Exit(1)
	}

	c, err := config.ReadDefault(*configFile)
	if nil != err {
		port = DEFAULT_PORT
	}
	port, err = c.String("service", "port")
	if nil != err {
		port = DEFAULT_PORT
	}
	/*
		logfile, err = c.String("service", "logfile")
		if nil != err {
			logfile = DEFAULT_LOG
		}

		logfile := DEFAULT_LOG
		os.MkdirAll(filepath.Dir(logfile), 0777)
		f, err := os.OpenFile(logfile, os.O_RDWR|os.O_CREATE, 0666)
		log.SetOutput(f)
	*/
	timeoutSeconds = 600
	go func() {
		for arg := range timeoutChan {
			func() {
				verifyCodeTimeoutLock.Lock()
				defer verifyCodeTimeoutLock.Unlock()
				//如果用户在有效期内多次点击发送验证码，最新的验证码会覆盖旧的验证码，此时，旧验证码超时通知时，不应该删除最新的验证码。
				if int(time.Now().Sub(verifyCodeTimeout[arg.id].heartbeat).Seconds()) >= timeoutSeconds {
					fmt.Printf("verify code timeout: %s:%s.\n", arg.id, verifyCodeTimeout[arg.id].vcode)
					delete(verifyCodeTimeout, arg.id)
				}
			}()
		}
	}()

	e := echo.New()
	EchoInit(e)
	e.Use(emw.CORSWithConfig(emw.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
		AllowMethods: []string{echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE},
	}))

	v1 := e.Group("/v1")
	v1.POST("/register", Register)
	v1.POST("/signin", SignIn)
	v1.POST("/signincold", SignInCold)
	v1.POST("/verifycode", SendVerifyCodeMock)
	v1.POST("/userinfo", GetUserInfo)
	v1.POST("/userinfoall", GetUserInfoAll)
	v1.POST("/applyinfoall", GetUserApplyAll)
	v1.POST("/uploadidcard", UploadIDCard)
	v1.POST("/uploadbankcard", UploadBankCard)
	v1.POST("/updateverifyok", UpdateVerifyok)

	v1.POST("/deletebankcard", DeleteBankCard)
	v1.POST("/signout", SignOut)
	v1.POST("/setassetinfo", SetUserAssetInfo)
	v1.POST("/getassetinfo", GetUserAssetInfo)
	v1.POST("/setapplyinfo", SetUserApplyInfo)
	v1.POST("/setapplystatus", SetApplyStatus)
	v1.POST("/getapplyinfo", GetUserApplyInfo)
	v1.POST("/getapplyinfobyid", GetUserApplyInfoByID)
	v1.POST("/setrecordinfo", SetUserRecordInfo)
	v1.POST("/getrecordinfo", GetUserRecordInfo)
	v1.POST("/deleteassetaccount", DeleteAssetAccount)
	v1.POST("/getcoinprice", GetCoinPrice)

	v1.Static("/images", "images")

	srvAddr := ":" + port

	log.Printf("Listening and serving HTTP on %s\n", srvAddr)
	// Start server
	go func() {
		if err := e.Start(srvAddr); err != nil {
			log.Fatal("shutting down the server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 10 seconds.
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGUSR1, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
	log.Println("Server exit")

}
