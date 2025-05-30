package v1

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/1Panel-dev/1Panel/backend/app/api/v1/helper"
	"github.com/1Panel-dev/1Panel/backend/constant"
	"github.com/1Panel-dev/1Panel/backend/global"
	"github.com/1Panel-dev/1Panel/backend/middleware"
	"github.com/gin-gonic/gin"
)

// @Tags MHosts
// @Summary 将请求转发
// @Accept json
// @Success 200
// @Security ApiKeyAuth
// @Security Timestamp
// @Router /mhosts/forward [post]
func (b *BaseApi) ForwardHostApi(c *gin.Context) {
	body := strings.NewReader(``)

	req, err := http.NewRequest("POST", "http://localhost:37688/api/v1/apps/search", body)

	ts := strconv.Itoa(int(time.Now().UnixMilli() / 1000))
	token := middleware.GenerateMD5("1panel" + global.CONF.System.ApiKey + ts)
	req.Header["1Panel-Timestamp"] = []string{ts}
	req.Header["1Panel-Token"] = []string{token}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		helper.ErrorWithDetail(c, constant.CodeErrInternalServer, constant.ErrTypeInternalServer, err)
		return
	}

	bodyBuf, err := io.ReadAll(resp.Body)
	if err != nil {
		helper.ErrorWithDetail(c, constant.CodeErrInternalServer, constant.ErrTypeInternalServer, err)
		return
	}

	global.LOG.Infof("bodyBuf: %+v", string(bodyBuf))

	helper.SuccessWithData(c, nil)
}
