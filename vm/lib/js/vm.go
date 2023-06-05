package js

import (
	"github.com/mangalorg/libmangal/vm/util"
	"github.com/robertkrimen/otto"
	lua "github.com/yuin/gopher-lua"
)

const vmTypeName = "js_vm"

func pushVM(L *lua.LState, vm *otto.Otto) {
	ud := L.NewUserData()
	ud.Value = vm
	L.SetMetatable(ud, L.GetTypeMetatable(vmTypeName))
	L.Push(ud)
}

func checkVM(L *lua.LState, n int) *otto.Otto {
	ud := L.CheckUserData(n)
	if v, ok := ud.Value.(*otto.Otto); ok {
		return v
	}
	L.ArgError(n, "js_vm expected")
	return nil
}

func newVM(L *lua.LState) int {
	vm := otto.New()
	pushVM(L, vm)
	return 1
}

func vmRun(L *lua.LState) int {
	vm := checkVM(L, 1)
	script := L.CheckString(2)

	value, err := vm.Run(script)
	if err != nil {
		L.RaiseError(err.Error())
		return 0
	}

	pushVMValue(L, &value)
	return 1
}

func vmGet(L *lua.LState) int {
	vm := checkVM(L, 1)
	name := L.CheckString(2)

	value, err := vm.Get(name)
	if err != nil {
		L.RaiseError(err.Error())
		return 0
	}

	pushVMValue(L, &value)
	return 1
}

func vmSet(L *lua.LState) int {
	vm := checkVM(L, 1)
	name := L.CheckString(2)
	lvalue := L.CheckAny(3)

	value, err := util.FromLValue(lvalue)
	if err != nil {
		L.Push(lua.LString(err.Error()))
		L.RaiseError(err.Error())
		return 0
	}

	err = vm.Set(name, value)
	if err != nil {
		L.RaiseError(err.Error())
		return 0
	}

	return 0
}
