package http

import (
	luadoc "github.com/mangalorg/libmangal/vm/doc"
	"github.com/philippgille/gokv"
	lua "github.com/yuin/gopher-lua"
	"net/http"
)

type LibOptions struct {
	HTTPClient *http.Client
	HTTPStore  gokv.Store
}

func Lib(options LibOptions) *luadoc.Lib {
	classRequest := &luadoc.Class{
		Name:        requestTypeName,
		Description: "HTTP Request",
		Methods: []*luadoc.Method{
			{
				Name:        "header",
				Description: "Get or set request header",
				Value:       requestHeader,
				Params: []*luadoc.Param{
					{
						Name:        "key",
						Description: "Header key",
						Type:        luadoc.String,
					},
					{
						Name:        "value",
						Description: "Header value",
						Type:        luadoc.String,
						Optional:    true,
					},
				},
				Returns: []*luadoc.Param{
					{
						Name:        "value",
						Description: "Header value",
						Type:        luadoc.String,
						Optional:    true,
					},
				},
			},
			{
				Name:        "cookie",
				Description: "Get or set request cookie",
				Value:       requestCookie,
				Params: []*luadoc.Param{
					{
						Name:        "key",
						Description: "Cookie key",
						Type:        luadoc.String,
					},
					{
						Name:        "value",
						Description: "Cookie value",
						Type:        luadoc.String,
						Optional:    true,
					},
				},
				Returns: []*luadoc.Param{
					{
						Name:        "value",
						Description: "Cookie value",
						Type:        luadoc.String,
						Optional:    true,
					},
				},
			},
			{
				Name:        "content_length",
				Description: "Get or set request content length",
				Value:       requestContentLength,
				Params: []*luadoc.Param{
					{
						Name:        "length",
						Description: "Content length",
						Type:        luadoc.Number,
						Optional:    true,
					},
				},
				Returns: []*luadoc.Param{
					{
						Name:        "length",
						Description: "Content length",
						Type:        luadoc.Number,
						Optional:    true,
					},
				},
			},
			{
				Name:        "send",
				Description: "Perform request",
				Value: func(L *lua.LState) int {
					return requestSend(L, options.HTTPClient, options.HTTPStore)
				},
				Returns: []*luadoc.Param{
					{
						Name:        "response",
						Description: "Response",
						Type:        responseTypeName,
					},
				},
			},
		},
	}

	classResponse := &luadoc.Class{
		Name:        responseTypeName,
		Description: "HTTP Response",
		Methods: []*luadoc.Method{
			{
				Name:        "header",
				Description: "Get response header",
				Value:       responseHeader,
				Params: []*luadoc.Param{
					{
						Name:        "key",
						Description: "Header key",
						Type:        luadoc.String,
					},
				},
				Returns: []*luadoc.Param{
					{
						Name:        "value",
						Description: "Header value",
						Type:        luadoc.String,
					},
				},
			},
			{
				Name:        "cookies",
				Description: `Get response cookies. Returns a list of cookies.`,
				Value:       responseCookies,
				Returns: []*luadoc.Param{
					{
						Name:        "cookies",
						Description: "Cookies",
						Type: luadoc.List(luadoc.TableLiteral(
							"name", luadoc.String,
							"value", luadoc.String,
						)),
					},
				},
			},
			{
				Name:        "content_length",
				Description: "Get response content length",
				Value:       responseContentLength,
				Returns: []*luadoc.Param{
					{
						Name:        "length",
						Description: "Content length",
						Type:        luadoc.Number,
					},
				},
			},
			{
				Name:        "body",
				Description: "Get response body",
				Value:       responseBody,
				Returns: []*luadoc.Param{
					{
						Name:        "body",
						Description: "Body",
						Type:        luadoc.String,
					},
				},
			},
			{
				Name:        "status",
				Description: "Get response status",
				Value:       responseStatus,
				Returns: []*luadoc.Param{
					{
						Name:        "status",
						Description: "Status",
						Type:        luadoc.Number,
					},
				},
			},
		},
	}

	return &luadoc.Lib{
		Name:        "http",
		Description: "Package http provides HTTP client implementations. Make HTTP (or HTTPS) requests",
		Vars: []*luadoc.Var{
			{
				Name:        "MethodGet",
				Description: "GET HTTP Method",
				Value:       lua.LString(http.MethodGet),
				Type:        luadoc.String,
			},
			{
				Name:        "MethodHead",
				Description: "HEAD HTTP Method",
				Value:       lua.LString(http.MethodHead),
				Type:        luadoc.String,
			},
			{
				Name:        "MethodPost",
				Description: "POST HTTP Method",
				Value:       lua.LString(http.MethodPost),
				Type:        luadoc.String,
			},
			{
				Name:        "MethodPut",
				Description: "PUT HTTP Method",
				Value:       lua.LString(http.MethodPut),
				Type:        luadoc.String,
			},
			{
				Name:        "MethodPatch",
				Description: "PATCH HTTP Method",
				Value:       lua.LString(http.MethodPatch),
				Type:        luadoc.String,
			},
			{
				Name:        "MethodDelete",
				Description: "DELETE HTTP Method",
				Value:       lua.LString(http.MethodDelete),
				Type:        luadoc.String,
			},
			{
				Name:        "MethodConnect",
				Description: "CONNECT HTTP Method",
				Value:       lua.LString(http.MethodConnect),
				Type:        luadoc.String,
			},
			{
				Name:        "StatusOK",
				Description: `StatusOK is returned if the request was successful`,
				Value:       lua.LNumber(http.StatusOK),
				Type:        luadoc.Number,
			},
		},
		Classes: []*luadoc.Class{classRequest, classResponse},
		Funcs: []*luadoc.Func{
			{
				Name:        "request",
				Description: "Create a new HTTP request",
				Value:       requestNew,
				Params: []*luadoc.Param{
					{
						Name:        "method",
						Description: "HTTP method",
						Type: luadoc.Enum(
							http.MethodGet,
							http.MethodHead,
							http.MethodPost,
							http.MethodPut,
							http.MethodPatch,
							http.MethodDelete,
							http.MethodConnect,
						),
					},
					{
						Name:        "url",
						Description: "URL",
						Type:        luadoc.String,
					},
					{
						Name:        "body",
						Description: "Request body",
						Type:        luadoc.String,
						Optional:    true,
					},
				},
				Returns: []*luadoc.Param{
					{
						Name:        "request",
						Description: "Request",
						Type:        requestTypeName,
					},
				},
			},
		},
	}
}
