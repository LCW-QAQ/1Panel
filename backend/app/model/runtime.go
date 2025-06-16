package model

import (
	"path"

	"github.com/1Panel-dev/1Panel/backend/constant"
)

type Runtime struct {
	BaseModel
	Name          string `gorm:"type:varchar(255);not null" json:"name"`
	AppDetailID   uint   `gorm:"type:integer" json:"appDetailId"`
	Image         string `gorm:"type:varchar(255)" json:"image"`
	WorkDir       string `gorm:"type:varchar(255)" json:"workDir"`
	DockerCompose string `gorm:"type:text" json:"dockerCompose"`
	Env           string `gorm:"type:varchar(255)" json:"env"`
	Params        string `gorm:"type:text" json:"params"`
	Version       string `gorm:"type:varchar(255);not null" json:"version"`
	Type          string `gorm:"type:varchar(255);not null" json:"type"`
	Status        string `gorm:"type:varchar(255);not null" json:"status"`
	Resource      string `gorm:"type:varchar(255);not null" json:"resource"`
	Port          int    `gorm:"type:integer;" json:"port"`
	Message       string `gorm:"type:longtext;" json:"message"`
	CodeDir       string `gorm:"type:varchar(255);" json:"codeDir"`
}

func (r *Runtime) GetComposePath() string {
	return path.Join(r.GetPath(), "docker-compose.yml")
}

func (r *Runtime) GetEnvPath() string {
	return path.Join(r.GetPath(), ".env")
}

func (r *Runtime) GetPath() string {
	return path.Join(constant.RuntimeDir, r.Type, r.Name)
}

func (r *Runtime) GetLogPath() string {
	return path.Join(r.GetPath(), "build.log")
}
