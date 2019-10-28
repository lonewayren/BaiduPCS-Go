package baidupcs

import (
	"errors"
	"github.com/iikira/BaiduPCS-Go/baidupcs/pcserror"
	"github.com/PuerkitoBio/goquery"
	"regexp"
	"strconv"
)

type (
	// ShareOption 分享可选项
	ShareOption struct {
		Password string // 密码
		Period   int    // 有效期
	}

	// Shared 分享信息
	Shared struct {
		Link    string `json:"link"`
		ShareID int64  `json:"shareid"`
	}

	// ShareRecordInfo 分享信息
	ShareRecordInfo struct {
		ShareID         int64   `json:"shareId"`
		FsIds           []int64 `json:"fsIds"`
		Passwd          string  `json:"passwd"`
		Shortlink       string  `json:"shortlink"`
		Status          int     `json:"status"`          // 状态
		TypicalCategory int     `json:"typicalCategory"` // 文件类型
		TypicalPath     string  `json:"typicalPath"`
	}

	sharePSetJSON struct {
		*Shared
		*pcserror.PanErrorInfo
	}

	shareListJSON struct {
		List ShareRecordInfoList `json:"list"`
		*pcserror.PanErrorInfo
	}

	// ShareFileInfo 分享文件信息
	ShareFileInfo struct {
		SourceID		string	`json:"sourceId"`
		ShareID         int64   `json:"shareId"`
		FsIds           []int64 `json:"fsIds"`
		ShareUK			int64   `json:"shareUk"` // 文件类型
	}

	transferJSON struct {
		ErrorNo	int	`json:"errno"`
		*pcserror.PanErrorInfo
	}
)

var (
	// ErrShareLinkNotFound 未找到分享链接
	ErrShareLinkNotFound = errors.New("未找到分享链接")
)

// Clean 清理
func (sri *ShareRecordInfo) Clean() {
	if sri.Passwd == "0" {
		sri.Passwd = ""
	}
}

// HasPasswd 是否需要提取码
func (sri *ShareRecordInfo) HasPasswd() bool {
	return sri.Passwd != "" && sri.Passwd != "0"
}

// ShareRecordInfoList 分享信息列表
type ShareRecordInfoList []*ShareRecordInfo

// Clean 清理
func (sril *ShareRecordInfoList) Clean() {
	for _, sri := range *sril {
		if sri == nil {
			continue
		}

		sri.Clean()
	}
}

// ShareSet 分享文件
func (pcs *BaiduPCS) ShareSet(paths []string, option *ShareOption) (s *Shared, pcsError pcserror.Error) {
	if option == nil {
		option = &ShareOption{}
	}

	dataReadCloser, pcsError := pcs.PrepareSharePSet(paths, option.Period, option.Password)
	if pcsError != nil {
		return
	}

	defer dataReadCloser.Close()

	errInfo := pcserror.NewPanErrorInfo(OperationShareSet)
	jsonData := sharePSetJSON{
		Shared:       &Shared{},
		PanErrorInfo: errInfo,
	}

	pcsError = pcserror.HandleJSONParse(OperationShareSet, dataReadCloser, &jsonData)
	if pcsError != nil {
		return
	}

	if jsonData.Link == "" {
		errInfo.ErrType = pcserror.ErrTypeOthers
		errInfo.Err = ErrShareLinkNotFound
		return nil, errInfo
	}

	return jsonData.Shared, nil
}

// ShareSet 分享文件
func (pcs *BaiduPCS) ShareInfo(sourceID string, pwd string) (v bool, pcsError pcserror.Error) {
	var valid bool
	resp, dataReadCloser, pcsError := pcs.PrepareShareVerify(sourceID, pwd)

	if pcsError != nil {
		return
	}

	defer dataReadCloser.Close()
	errInfo := pcserror.NewPanErrorInfo(OperationShareVarify)
	var BDCLND string
	cookie := resp.Cookies()
	for i:=0 ;i < len(cookie); i++ {
		if cookie[i].Name == "BDCLND" {
			BDCLND = cookie[i].Value
		}
	}

	if BDCLND == "" {
		errInfo.ErrType = pcserror.ErrTypeOthers
		errInfo.Err = ErrShareLinkNotFound
		return false, errInfo
	} else {
		valid = true
	}
	return valid, nil
}

// ShareSet 分享文件
func (pcs *BaiduPCS) ShareVarify(sourceID string, pwd string) (v bool, pcsError pcserror.Error) {
	var valid bool
	resp, dataReadCloser, pcsError := pcs.PrepareShareVerify(sourceID, pwd)

	if pcsError != nil {
		return
	}

	defer dataReadCloser.Close()
	errInfo := pcserror.NewPanErrorInfo(OperationShareVarify)
	var BDCLND string
	cookie := resp.Cookies()
	for i:=0 ;i < len(cookie); i++ {
		if cookie[i].Name == "BDCLND" {
			BDCLND = cookie[i].Value
		}
	}

	if BDCLND == "" {
		errInfo.ErrType = pcserror.ErrTypeOthers
		errInfo.Err = ErrShareLinkNotFound
		return false, errInfo
	} else {
		valid = true
	}
	return valid, nil
}


// ShareParse 解析分享文件信息
func (pcs *BaiduPCS) ShareParse(sourceID string, pwd string) (shareFileInfo ShareFileInfo, pcsError pcserror.Error) {
	shareFileInfo.SourceID = sourceID
	_, dataReadCloser, pcsError := pcs.PrepareShareParse(sourceID, pwd)

	if pcsError != nil {
		return
	}

	errInfo := pcserror.NewPanErrorInfo(OperationShareVarify)
	docs, err := goquery.NewDocumentFromReader(dataReadCloser)

	if err != nil {
		return shareFileInfo, errInfo
	}
	defer dataReadCloser.Close()
	var matched bool
	var content string
	docs.Find("html").Find("body").Find("script").Each(func(i int, selection *goquery.Selection) {
		matched, _ = regexp.MatchString("yunData\\.setData\\(\\{", selection.Text())
		if matched {
			content = selection.Text()
		}
	})
	if matched {
		re, _ := regexp.Compile("\"uk\":([0-9]+),")
		matchedValues := re.FindStringSubmatch(content)
		if len(matchedValues) >= 2 {
			shareFileInfo.ShareUK, _ = strconv.ParseInt(matchedValues[1], 10, 64)
		}
		re, _ = regexp.Compile("\"shareid\":([0-9]+),")
		matchedValues = re.FindStringSubmatch(content)
		if len(matchedValues) >= 2 {
			shareFileInfo.ShareID, _ = strconv.ParseInt(matchedValues[1], 10, 64)
		}
		re, _ = regexp.Compile("\"fs_id\":([0-9]+),")
		matchedValueList := re.FindAllStringSubmatch(content, -1)
		if len(matchedValueList) >= 1 {
			var FsIds []int64
			for i := 0; i < len(matchedValueList); i++ {
				fs_id, _ := strconv.ParseInt(matchedValueList[i][1], 10, 64)
				FsIds = append(FsIds, fs_id)
			}
			shareFileInfo.FsIds = removeDuplicateElement(FsIds[:])
		}
	}
	if ! matched {
		errInfo.ErrType = pcserror.ErrTypeOthers
		errInfo.Err = ErrShareLinkNotFound
		return shareFileInfo, errInfo
	}

	return shareFileInfo, nil
}

// ShareParse 解析分享文件信息
func (pcs *BaiduPCS) ShareTransfer(sourceID string, pwd string, path string) (ErrorNo int, pcsError pcserror.Error) {

	shareFileInfo := ShareFileInfo{SourceID:sourceID}

	resp, dataReadCloser, pcsError := pcs.PrepareShareParse(sourceID, pwd)

	if pcsError != nil {
		return 1000, pcsError
	}

	errInfo := pcserror.NewPanErrorInfo(OperationShareParse)
	jsonData := transferJSON{
		ErrorNo: 1000,
		PanErrorInfo: errInfo,
	}
	docs, err := goquery.NewDocumentFromReader(dataReadCloser)

	if err != nil {
		return jsonData.ErrorNo, errInfo
	}

	defer dataReadCloser.Close()
	var matched bool
	var content string
	docs.Find("html").Find("body").Find("script").Each(func(i int, selection *goquery.Selection) {
		matched, _ = regexp.MatchString("yunData\\.setData\\(\\{", selection.Text())
		if matched {
			content = selection.Text()
		}
	})
	if matched {
		re, _ := regexp.Compile("\"uk\":([0-9]+),")
		matchedValues := re.FindStringSubmatch(content)
		if len(matchedValues) >= 2 {
			shareFileInfo.ShareUK, _ = strconv.ParseInt(matchedValues[1], 10, 64)
		}
		re, _ = regexp.Compile("\"shareid\":([0-9]+),")
		matchedValues = re.FindStringSubmatch(content)
		if len(matchedValues) >= 2 {
			shareFileInfo.ShareID, _ = strconv.ParseInt(matchedValues[1], 10, 64)
		}
		re, _ = regexp.Compile("\"fs_id\":([0-9]+),")
		matchedValueList := re.FindAllStringSubmatch(content, -1)
		if len(matchedValueList) >= 1 {
			var FsIds []int64
			for i := 0; i < len(matchedValueList); i++ {
				fs_id, _ := strconv.ParseInt(matchedValueList[i][1], 10, 64)
				FsIds = append(FsIds, fs_id)
			}
			shareFileInfo.FsIds = removeDuplicateElement(FsIds[:])
		}
	} else {
		errInfo.ErrType = pcserror.ErrTypeOthers
		errInfo.Err = ErrShareLinkNotFound
		return jsonData.ErrorNo, errInfo
	}
	pcs.client.Jar.SetCookies(resp.Request.URL, resp.Cookies())
	dataReadCloser, pcsError = pcs.PrepareShareTransfer(shareFileInfo, path)

	pcsError = pcserror.HandleJSONParse(OperationShareTransfer, dataReadCloser, &jsonData)
	if pcsError != nil {
		return
	}

	return jsonData.ErrorNo, nil
}


// ShareCancel 取消分享
func (pcs *BaiduPCS) ShareCancel(shareIDs []int64) (pcsError pcserror.Error) {
	dataReadCloser, pcsError := pcs.PrepareShareCancel(shareIDs)
	if pcsError != nil {
		return
	}

	defer dataReadCloser.Close()

	pcsError = pcserror.DecodePanJSONError(OperationShareCancel, dataReadCloser)
	return
}

// ShareList 列出分享列表
func (pcs *BaiduPCS) ShareList(page int) (records ShareRecordInfoList, pcsError pcserror.Error) {
	dataReadCloser, pcsError := pcs.PrepareShareList(page)
	if pcsError != nil {
		return
	}

	defer dataReadCloser.Close()

	errInfo := pcserror.NewPanErrorInfo(OperationShareList)
	jsonData := shareListJSON{
		List:         records,
		PanErrorInfo: errInfo,
	}

	pcsError = pcserror.HandleJSONParse(OperationShareList, dataReadCloser, &jsonData)
	if pcsError != nil {
		return
	}

	if jsonData.List == nil {
		errInfo.ErrType = pcserror.ErrTypeOthers
		errInfo.Err = errors.New("shared list is nil")
		return nil, errInfo
	}

	jsonData.List.Clean()
	return jsonData.List, nil
}
