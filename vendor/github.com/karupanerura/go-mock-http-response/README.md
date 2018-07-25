# go-mock-http-response

Simple http client mock tool.

[![Build Status](https://travis-ci.org/karupanerura/go-mock-http-response.svg?branch=master)](https://travis-ci.org/karupanerura/go-mock-http-response)
[![codecov](https://codecov.io/gh/karupanerura/go-mock-http-response/branch/master/graph/badge.svg)](https://codecov.io/gh/karupanerura/go-mock-http-response)
[![GoDoc](https://godoc.org/github.com/karupanerura/go-mock-http-response?status.svg)](http://godoc.org/github.com/karupanerura/go-mock-http-response)

## SYNOPSIS

```go
package main

import (
    "io/ioutil"
    "net/http"
    "testing"

    mockhttp "github.com/karupanerura/go-mock-http-response"
)

func mockResponse(statusCode int, headers map[string]string, body []byte) {
    http.DefaultClient = mockhttp.NewResponseMock(statusCode, headers, body).MakeClient()
}

func checkFoo() (bool, error) {
    res, err := http.Get("http://example.com/")
    if err != nil {
        return false, err
    }
    defer res.Body.Close()

    body, err := ioutil.ReadAll(res.Body)
    if err != nil {
        return false, err
    }

    isFoo := string(body) == "foo"
    return isFoo, nil
}

func TestFoo(t *testing.T) {
    mockResponse(http.StatusOK, map[string]string{"Content-Type": "text/plain"}, []byte("foo"))
    isFoo, err := checkFoo()
    if err != nil {
        t.Fatal(err)
    }
    if isFoo != true {
        t.Errorf("Should be true, but got false")
    }

    mockResponse(http.StatusOK, map[string]string{"Content-Type": "text/plain"}, []byte("bar"))
    isFoo, err = checkFoo()
    if err != nil {
        t.Fatal(err)
    }
    if isFoo != false {
        t.Errorf("Should be false, but got true")
    }
}
```
