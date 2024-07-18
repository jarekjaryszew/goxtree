package goxtree

import (
	"fmt"
	"syscall/js"
)

type FetchConfig struct {
	Method  string
	Headers map[string]string
	Body    string
}

func Fetch(url string, callback func(string), config *FetchConfig) {
	fetch := js.Global().Get("fetch")

	var getter = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		awaitable := fetch.Invoke(url)
		ch := make(chan []js.Value)
		cb := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			ch <- args
			return nil
		})
		awaitable.Call("then", cb)
		go func() {
			results := <-ch
			fmt.Println("response", results)
			rsp := results[0]
			awaitable = rsp.Call("text")
			go awaitable.Call("then", cb)
			results = <-ch
			fmt.Println("text1", results)
			resStr := results[0].String()
			fmt.Println("text2", resStr)
			callback(resStr)
		}()
		return nil
	})
	getter.Invoke()
}
