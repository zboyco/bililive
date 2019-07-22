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

func startWeb(m *memberModel, point *string, run *bool) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Static("/static", "public")
	r.LoadHTMLGlob("templates/*")
	r.GET("/api/set", func(c *gin.Context) {
		*point = c.Query("point")

		c.JSON(http.StatusOK, nil)
	})
	r.GET("/api/start", func(c *gin.Context) {
		*run = true
		m.Lock()
		m.body = make(map[string]bool)
		m.arr = make([]string, 0)
		m.Unlock()

		c.JSON(http.StatusOK, nil)
	})
	r.GET("/api/stop", func(c *gin.Context) {
		*run = false

		c.JSON(http.StatusOK, nil)
	})
	r.GET("/api/members", func(c *gin.Context) {
		m.Lock()
		result := m.arr
		m.arr = make([]string, 0)
		m.Unlock()

		c.JSON(http.StatusOK, gin.H{
			"members": result,
		})
	})
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	r.Run()
}
