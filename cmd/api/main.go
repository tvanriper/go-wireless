package main

import (
	"github.com/gin-gonic/gin"
	"github.com/tvanriper/go-wireless/api"
)

func main() {
	r := gin.Default()
	api.SetupRoutes(r)
	r.Run(":8080")
}
