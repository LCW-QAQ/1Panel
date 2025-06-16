package model

type Setting struct {
	BaseModel
	Key   string `json:"key" gorm:"column:key_;type:varchar(256);not null;"`
	Value string `json:"value" gorm:"type:text"`
	About string `json:"about" gorm:"type:longText"`
}
