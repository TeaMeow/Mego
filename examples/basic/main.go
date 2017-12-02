package main

import mego "github.com/TeaMeow/Mego"

func main() {
	// 建立一個 Mego 引擎。
	e := mego.New()

	// Sum - 加總
	//
	// 參數：A int, B int
	// 說明：將客戶端傳入的 A, B 參數進行加總並回傳其結果。
	e.Register("Sum", func(c *mego.Context) {
		a, _ := c.Params.GetInt(0)
		b, _ := c.Params.GetInt(1)
		c.Respond(mego.StatusOK, a+b)
	})

	// 在 :80 埠口上執行 Mego 引擎。
	e.Run()
}