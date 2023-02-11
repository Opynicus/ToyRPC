/*
 * @Author: Opynicus
 * @Date: 2023-02-11 15:00:28
 * @LastEditTime: 2023-02-11 15:49:36
 * @LastEditors: Opynicus
 * @Description:
 * @FilePath: \ToyRPC\service\service.go
 * 可以输入预定的版权声明、个性签名、空行等
 */
package service

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

type Service struct {
	name   string
	typ    reflect.Type
	rcvr   reflect.Value
	method map[string]*MethodType
}

func NewService(rcvr interface{}) *Service {
	s := new(Service)
	s.typ = reflect.TypeOf(rcvr)
	s.rcvr = reflect.ValueOf(rcvr)
	s.name = reflect.Indirect(s.rcvr).Type().Name()
	if !ast.IsExported(s.name) {
		log.Fatalf("rpc server: %s is not a valid service name", s.name)
	}
	s.method = suitableMethods(s.typ)
	return s
}

func suitableMethods(typ reflect.Type) map[string]*MethodType {
	methods := make(map[string]*MethodType)
	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)
		mtype := method.Type
		mname := method.Name
		if method.PkgPath != "" {
			continue // method must be exported
		}
		if mtype.NumIn() != 3 || mtype.NumOut() != 1 {
			continue
		}
		argType := mtype.In(1)
		replyType := mtype.In(2)
		if !isExportedOrBuiltinType(replyType) && !isExportedOrBuiltinType(argType) {
			continue
		}
		if mtype.NumOut() != 1 {
			continue
		}
		methods[mname] = &MethodType{method: method, ArgType: argType, ReplyType: replyType}
	}
	return methods
}

func (s *Service) call(m *MethodType, argv, replyv reflect.Value) error {
	atomic.AddUint64(&m.numsCall, 1)
	f := m.method.Func
	returnValues := f.Call([]reflect.Value{s.rcvr, argv, replyv})
	if errInter := returnValues[0].Interface(); errInter != nil {
		return errInter.(error)
	}
	return nil
}

type MethodType struct {
	method    reflect.Method
	ArgType   reflect.Type
	ReplyType reflect.Type
	numsCall  uint64
}

func (m *MethodType) NumCalls() uint64 {
	return atomic.LoadUint64(&m.numsCall)
}

func (m *MethodType) newArgv() reflect.Value {
	var argv reflect.Value
	if m.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(m.ArgType.Elem()) // type of argv is *T
	} else {
		argv = reflect.New(m.ArgType).Elem() // type of argv is T
	}
	return argv
}

func (m *MethodType) newReplyv() reflect.Value {
	replyv := reflect.New(m.ReplyType.Elem())
	switch m.ReplyType.Elem().Kind() {
	case reflect.Map:
		replyv.Elem().Set(reflect.MakeMap(m.ReplyType.Elem())) // type of replyv is *map[string]interface{}
	case reflect.Slice:
		replyv.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0)) // type of replyv is *[]interface{}
	}
	return replyv
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}
