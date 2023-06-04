package http

import (
	"bufio"
	"bytes"
	"github.com/philippgille/gokv"
	lua "github.com/yuin/gopher-lua"
	"net/http"
	"net/http/httputil"
	"strings"
)

const requestTypeName = "http_request"

func checkRequest(L *lua.LState, n int) *http.Request {
	ud := L.CheckUserData(n)
	if v, ok := ud.Value.(*http.Request); ok {
		return v
	}
	L.ArgError(1, "request expected")
	return nil
}

func pushRequest(L *lua.LState, request *http.Request) {
	ud := L.NewUserData()
	ud.Value = request
	L.SetMetatable(ud, L.GetTypeMetatable(requestTypeName))
	L.Push(ud)
}

func requestNew(L *lua.LState) int {
	method := L.CheckString(1)
	url := L.CheckString(2)
	body := L.OptString(3, "")

	request, err := http.NewRequestWithContext(
		L.Context(),
		method,
		url,
		strings.NewReader(body),
	)
	if err != nil {
		L.RaiseError(err.Error())
	}

	pushRequest(L, request)
	return 1
}

func requestHeader(L *lua.LState) int {
	request := checkRequest(L, 1)
	key := L.CheckString(2)

	if L.GetTop() == 3 {
		value := L.CheckString(3)
		request.Header.Set(key, value)
		return 0
	}

	L.Push(lua.LString(request.Header.Get(key)))
	return 1
}

func requestCookie(L *lua.LState) int {
	request := checkRequest(L, 1)
	key := L.CheckString(2)

	if L.GetTop() == 3 {
		value := L.CheckString(3)
		cookie := &http.Cookie{Name: key, Value: value}
		request.AddCookie(cookie)
		return 0
	}

	cookie, _ := request.Cookie(key)
	if cookie == nil {
		L.Push(lua.LNil)
		return 1
	}

	L.Push(lua.LString(cookie.Value))
	return 1
}

func requestContentLength(L *lua.LState) int {
	request := checkRequest(L, 1)

	if L.GetTop() == 2 {
		value := L.CheckInt64(2)
		request.ContentLength = value
		return 0
	}

	L.Push(lua.LNumber(request.ContentLength))
	return 1
}

func requestSend(L *lua.LState, client *http.Client, store gokv.Store) int {
	request := checkRequest(L, 1)

	dumpedRequest, errRequestDump := httputil.DumpRequestOut(request, true)
	dumpedRequestString := string(dumpedRequest)

	if errRequestDump == nil {
		var dumpedResponse []byte

		found, err := store.Get(dumpedRequestString, &dumpedResponse)
		if err != nil {
			_ = store.Delete(dumpedRequestString)
			goto doRequest
		}

		if !found {
			goto doRequest
		}

		response, err := http.ReadResponse(
			bufio.NewReader(bytes.NewReader(dumpedResponse)),
			request,
		)

		if err != nil {
			_ = store.Delete(dumpedRequestString)
			goto doRequest
		}

		pushResponse(L, response)
		return 1
	}

doRequest:
	response, err := client.Do(request)
	if err != nil {
		L.RaiseError(err.Error())
		return 0
	}

	if errRequestDump == nil {
		dumpedResponse, err := httputil.DumpResponse(response, true)
		if err != nil {
			goto exit
		}

		_ = store.Set(dumpedRequestString, dumpedResponse)
	}

exit:
	pushResponse(L, response)
	return 1
}
