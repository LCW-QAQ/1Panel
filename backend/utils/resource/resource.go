package resource

import (
	"path/filepath"
	"strings"

	"github.com/1Panel-dev/1Panel/backend/global"
)

func WarpDownloadUrl(url string) string {
	url = filepath.ToSlash(url)
	baseIndex := strings.Index(url, filepath.Join(global.CONF.System.Mode, "1panel"))
	if baseIndex < 0 || baseIndex >= len(url) {
		return ""
	}
	return global.CONF.System.AppRepo + "/" + url[baseIndex:]
}
