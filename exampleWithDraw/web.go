package main

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

type memberModel struct {
	body map[string]bool
	arr  []string
	sync.Mutex
}

func startWeb(m *memberModel) {
	r := gin.Default()
	r.Static("/static", "public")
	r.LoadHTMLGlob("templates/*")
	r.GET("/api/members", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"members": m.arr,
		})
	})
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	r.Run()
}
