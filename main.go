package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Server struct {
	db     *gorm.DB
	router *gin.Engine
}

func NewServer() (*Server, error) {
	db, err := initDB()
	if err != nil {
		return nil, err
	}

	return &Server{
		db:     db,
		router: gin.Default(),
	}, nil
}

func main() {
	s, err := NewServer()
	if err != nil {
		log.Fatal(err)
	}

	if err := s.router.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
