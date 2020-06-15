package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func startWeb(m *memberModel, point *string, run *bool) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Static("/static", "./public")
	r.Static("/html", "./templates")
	r.GET("/api/set", func(c *gin.Context) {
		*point = c.Query("point")

		c.JSON(http.StatusOK, nil)
	})
	r.GET("/api/start", func(c *gin.Context) {
		*run = true
		m.Reset()
		c.JSON(http.StatusOK, nil)
	})
	r.GET("/api/stop", func(c *gin.Context) {
		*run = false

		c.JSON(http.StatusOK, nil)
	})
	r.GET("/api/members", func(c *gin.Context) {

		result := m.Pick()

		c.JSON(http.StatusOK, gin.H{
			"members": result,
		})
	})

	r.Run()
}
