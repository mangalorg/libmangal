package http

import (
	lua "github.com/yuin/gopher-lua"
	"io"
	"net/http"
)

const responseTypeName = "http_response"

func checkResponse(L *lua.LState, n int) *http.Response {
	ud := L.CheckUserData(n)
	if v, ok := ud.Value.(*http.Response); ok {
		return v
	}
	L.ArgError(1, "response expected")
	return nil
}

func pushResponse(L *lua.LState, response *http.Response) {
	ud := L.NewUserData()
	ud.Value = response
	L.SetMetatable(ud, L.GetTypeMetatable(responseTypeName))
	L.Push(ud)
}

func responseStatus(L *lua.LState) int {
	response := checkResponse(L, 1)
	L.Push(lua.LNumber(response.StatusCode))
	return 1
}

func responseBody(L *lua.LState) int {
	response := checkResponse(L, 1)

	var (
		buffer []byte
		err    error
	)

	// check content length
	if response.ContentLength > 0 {
		buffer = make([]byte, response.ContentLength)
		_, err = io.ReadFull(response.Body, buffer)
	} else {
		buffer, err = io.ReadAll(response.Body)
	}

	if err != nil {
		L.RaiseError("failed to read response body: %s", err.Error())
		return 0
	}

	L.Push(lua.LString(buffer))
	return 1
}

func responseHeader(L *lua.LState) int {
	response := checkResponse(L, 1)
	key := L.CheckString(2)

	L.Push(lua.LString(response.Header.Get(key)))
	return 1
}

func responseCookies(L *lua.LState) int {
	response := checkResponse(L, 1)

	cookies := L.NewTable()
	for _, cookie := range response.Cookies() {
		c := L.NewTable()
		c.RawSetString("name", lua.LString(cookie.Name))
		c.RawSetString("value", lua.LString(cookie.Value))

		cookies.Append(c)
	}

	L.Push(cookies)
	return 1
}

func responseContentLength(L *lua.LState) int {
	response := checkResponse(L, 1)
	L.Push(lua.LNumber(response.ContentLength))
	return 1
}
