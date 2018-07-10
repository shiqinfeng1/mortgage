package main

import (
	"container/list"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	_ "log"
	//"net/http"
	//"net/url"
	_ "reflect"
	"sync"
	"time"
	"github.com/labstack/echo"
)

type SessionManager struct {
	Lock       sync.Mutex
	Smap       map[string]*list.Element
	SL         *list.List //gc
	Expires    int
}
type Session struct {
	Key     string
	Sid     string
	Expires int
	Value   interface{}
}

/*
说明：只在start中用，并已经加锁，因此这里不需要,否则会引起死锁
*/
func (sm *SessionManager) get(sid string) (Session, error) {
	if element, ok := sm.Smap[sid]; ok {
		sm.updateList(sid)
		return *(element.Value.(*Session)), nil
	}
	fmt.Printf("SessionManager.get can't find session by sid:%s\n", sid)
	return *sm.NewSession("", sid, ""), errors.New("can't find session by sid")
}

func (sm *SessionManager) NewSession(key string, sid string, value interface{}) *Session {
	fmt.Printf("\nNewSession. token='%s' key='%s' time:'%v'\n", sid,key,time.Now())
	return &Session{
		Key:     key,
		Sid:     sid,
		Expires: sm.Expires,
		Value:   value,
	}
}

func (sm *SessionManager) SessionStart(c echo.Context,token string) (session Session,err error) {
	sm.Lock.Lock()
	defer sm.Lock.Unlock()
	if token == "" {
		sid := sm.sessionId()
		session = *sm.NewSession("", sid, "")
		fmt.Printf("\n[SessionStart]NewSession. NEW sessionID: '%s' Expires= %d\n", sid,session.Expires)
	} else {
		session, err = sm.get(token)
		//fmt.Printf("\n[SessionStart]GET Old Session. sessionID= '%s' Expires= %d\n", token,session.Expires)
	}
	return 
}

//垃圾回收
func (sm *SessionManager) Listen() {
	sm.Lock.Lock()
	defer sm.Lock.Unlock()
	var n *list.Element
	for e := sm.SL.Front(); e != nil; e = n {
		n = e.Next()
		if e.Value.(*Session).Expires == 0 {
			fmt.Printf("\nSession timeout. token=%s time:%v\n", e.Value.(*Session).Sid,time.Now())
			delete(sm.Smap, e.Value.(*Session).Sid)
			sm.SL.Remove(e)
		} else {
			e.Value.(*Session).Expires--
			fmt.Printf("\nSession ageing. token=%s Expires:%v\n", e.Value.(*Session).Sid,e.Value.(*Session).Expires)
		}
	}

	time.AfterFunc(time.Duration(sm.Expires)*time.Second, func() { sm.Listen() })
}

func (sm *SessionManager) Get(sid string) (Session, error) {
	if element, ok := sm.Smap[sid]; ok {
		sm.updateList(sid)
		return *(element.Value.(*Session)), nil
	}
	fmt.Printf("SessionManager.get can't find session by sid:%s\n", sid)
	return *sm.NewSession("", "", ""), errors.New("can't find session by sid")
}

func (sm *SessionManager) GetByKey(key string) (Session, error) {
	sm.Lock.Lock()
	defer sm.Lock.Unlock()
	for _, element := range sm.Smap {
		if element.Value.(*Session).Key == key {
			sm.updateList(element.Value.(*Session).Sid)
			return *(element.Value.(*Session)), nil
		}
	}
	fmt.Println("can't find session by key")
	return *sm.NewSession("", "", ""), errors.New("can't find session by key")
}

func (sm *SessionManager) Set(key string, sid string) (isExits bool) {
	isExits = false
	if _, ok := sm.Smap[sid]; ok {
		sm.SL.Remove(sm.Smap[sid])
		isExits = true
	}

	element := sm.SL.PushBack(sm.NewSession(key, sid, ""))
	sm.Smap[sid] = element
	return
}

func (sm *SessionManager) Del(key string) (bl bool) {
	sm.Lock.Lock()
	defer sm.Lock.Unlock()
	bl = false
	for _, element := range sm.Smap {
		if element.Value.(*Session).Key == key {
			delete(sm.Smap, element.Value.(*Session).Sid)
			sm.SL.Remove(element)
			bl = true
		}
	}
	return
}

//private函数，Lock之后使用，不用也不能上锁，否则死锁
func (sm *SessionManager) updateList(sid string) {

	if element, ok := sm.Smap[sid]; ok {
		element.Value.(*Session).Expires = sm.Expires
		sm.SL.MoveToBack(element)
	}
}

func (sm *SessionManager) sessionId() string {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)
}
