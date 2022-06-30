// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package feishu

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/apis"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/keystone/models"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud/pkg/util/httputils"
)

const (
	//LiAuthUrl = "https://user-center-service-test.it.lixiangoa.com/feishu?callback="
	LiAuthUrl = "https://user-center-api.lixiangoa.com/feishu?callback="
	CoaUrl    = "https://coa-it.chehejia.com:9528"
)

func (drv *SFeishuOAuth2Driver) GetSsoRedirectUri(ctx context.Context, callbackUrl, state string) (string, error) {
	//req := map[string]string{
	//	"state":        state,
	//	"redirect_uri": callbackUrl,
	//}
	//urlStr := fmt.Sprintf("%s?%s", AuthUrl, jsonutils.Marshal(req).QueryString())
	urlStr := fmt.Sprintf("%s%s", LiAuthUrl, callbackUrl)
	return urlStr, nil
}

const (
	//LiAccessTokenUrl = "https://user-center-service-test.it.lixiangoa.com/token?code="
	//LiUserInfoUrl    = "https://user-center-service-test.it.lixiangoa.com/api/user?token="
	LiAccessTokenUrl = "https://user-center-api.lixiangoa.com/token?code="
	LiUserInfoUrl    = "https://user-center-api.lixiangoa.com/api/user?token="
)

func fetchLiAccessToken(ctx context.Context, code string) (*sAccessTokenData, error) {
	httpclient := httputils.GetDefaultClient()
	_, resp, err := httputils.JSONRequest(httpclient, ctx, httputils.GET, LiAccessTokenUrl+code, nil, nil, true)
	if err != nil {
		return nil, errors.Wrap(err, "request access token")
	}
	data := sAccessTokenData{}
	//err = resp.Unmarshal(&data, "data")
	err = resp.Unmarshal(&data)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal")
	}
	return &data, nil
}

type sLiUserInfoData struct {
	Name      string `json:"name"`
	AvatarURL string `json:"avatar"`
	Email     string `json:"email"`
	UserID    string `json:"uid"`
	Mobile    string `json:"mobile"`
	StaffId   int64  `json:"staff_id"`
}

func fetchLiUserInfo(ctx context.Context, accessToken string) (*sLiUserInfoData, error) {
	httpclient := httputils.GetDefaultClient()
	header := http.Header{}
	header.Set("Authorization", "Bearer "+accessToken)
	_, resp, err := httputils.JSONRequest(httpclient, ctx, httputils.GET, LiUserInfoUrl+accessToken, header, nil, true)
	if err != nil {
		return nil, errors.Wrap(err, "request access token")
	}
	data := sLiUserInfoData{}
	err = resp.Unmarshal(&data)
	if err != nil {
		return nil, errors.Wrap(err, "Unmarshal")
	}
	return &data, nil
}

type sOauthToken struct {
	TokenType   string `json:"token_type"`
	Expires     int64  `json:"expires_in"`
	AccessToken string `json:"access_token"`
}

func fetchOauthToken(ctx context.Context, body jsonutils.JSONObject) (*sOauthToken, error) {
	httpclient := httputils.GetDefaultClient()
	header := http.Header{}

	_, resp, err := httputils.JSONRequest(httpclient, ctx, httputils.POST, CoaUrl+"/oauth/token", header, body, true)
	if err != nil {
		return nil, errors.Wrap(err, "request access token")
	}
	data := sOauthToken{}
	err = resp.Unmarshal(&data)
	if err != nil {
		return nil, errors.Wrap(err, "Unmarshal")
	}
	return &data, nil
}

type sCoaUserInfoData struct {
	Name         string `json:"name"`
	DepartmentId int64  `json:"department_id"`
	Email        string `json:"email"`
	UserName     string `json:"user_name"`
	StaffId      int64  `json:"id"`
}

type sCoaDepInfoData struct {
	Id        int64  `json:"id"`
	Fullname  string `json:"fullname"`
	CompanyId int64  `json:"company_id"`
	ParentId  int64  `json:"parent_id"`
	IdPath    string `json:"id_path"`
	NamePath  string `json:"name_path"`
}

func fetchCoaUserInfo(ctx context.Context, accessToken string, id string) (*sCoaUserInfoData, error) {
	httpclient := httputils.GetDefaultClient()
	header := http.Header{}
	header.Set("Authorization", "Bearer "+accessToken)

	_, resp, err := httputils.JSONRequest(httpclient, ctx, httputils.GET, CoaUrl+"/api/info/user_by_id/"+id, header, nil, true)
	if err != nil {
		return nil, errors.Wrap(err, "request coa user")
	}
	code, err := resp.Int("code")
	if err != nil {
		return nil, errors.Wrap(err, "request coa user")
	}
	message, err := resp.GetString("message")
	if err != nil {
		return nil, errors.Wrap(err, "request coa user")
	}
	if code != 0 || message != "" {
		e := fmt.Sprintf("error: code %d, message '%s'", code, message)
		return nil, errors.Wrap(err, e)
	}
	data, err := resp.Get("data")
	if err != nil {
		return nil, errors.Wrap(err, "request coa user")
	}
	d := sCoaUserInfoData{}
	err = data.Unmarshal(&d)
	if err != nil {
		return nil, errors.Wrap(err, "Unmarshal")
	}
	return &d, nil
}

func fetchCoaDepartmentsInfo(ctx context.Context, accessToken string, dep string) (*sCoaDepInfoData, error) {
	httpclient := httputils.GetDefaultClient()
	header := http.Header{}
	header.Set("Authorization", "Bearer "+accessToken)

	_, resp, err := httputils.JSONRequest(httpclient, ctx, httputils.GET, CoaUrl+"/api/info/departments/"+dep, header, nil, true)
	if err != nil {
		return nil, errors.Wrap(err, "request coa dep")
	}
	code, err := resp.Int("code")
	if err != nil {
		return nil, errors.Wrap(err, "request coa dep")
	}
	message, err := resp.GetString("message")
	if err != nil {
		return nil, errors.Wrap(err, "request coa dep")
	}
	if code != 0 || message != "" {
		e := fmt.Sprintf("error: code %d, message '%s'", code, message)
		return nil, errors.Wrap(err, e)
	}
	data, err := resp.Get("data")
	if err != nil {
		return nil, errors.Wrap(err, "request coa dep")
	}
	d := sCoaDepInfoData{}
	err = data.Unmarshal(&d)
	if err != nil {
		return nil, errors.Wrap(err, "Unmarshal")
	}
	return &d, nil
}

func fetchProjectMetadata() (map[int64][]string, error) {
	var ret []db.SMetadata
	q := db.Metadata.Query().Equals("obj_type", models.ProjectManager.Keyword())
	q = q.Startswith("key", db.USER_TAG_PREFIX)
	err := db.FetchModelObjects(db.Metadata, q, &ret)
	if err != nil {
		if errors.Cause(err) != sql.ErrNoRows {
			log.Errorf("fetchProjectMetadata %s", err)
		}

		return nil, err
	}

	deps := make(map[int64][]string)
	for i := range ret {
		item := ret[i]

		dep, err := strconv.ParseInt(strings.TrimPrefix(item.Key, db.USER_TAG_PREFIX), 10, 64)
		if err != nil {
			log.Warningf("fetchProjectMetadata.ParseInt %s for dep %s", strings.TrimPrefix(item.Key, db.USER_TAG_PREFIX), err)
			continue
		}
		if _, ok := deps[dep]; !ok {
			deps[dep] = []string{item.ObjId}
			continue
		}
		deps[dep] = append(deps[dep], item.ObjId)
	}

	return deps, nil
}

func (drv *SFeishuOAuth2Driver) Authenticate(ctx context.Context, code string) (map[string][]string, error) {
	accessData, err := fetchLiAccessToken(ctx, code)
	if err != nil {
		return nil, errors.Wrap(err, "fetchAccessToken")
	}
	userInfo, err := fetchLiUserInfo(ctx, accessData.AccessToken)
	if err != nil {
		return nil, errors.Wrap(err, "fetchUserInfo")
	}
	coaModel, err := models.ServiceManager.FetchByName(models.GetDefaultAdminCred(), apis.SERVICE_TYPE_COA)
	if err != nil {
		return nil, errors.Wrap(err, "fetch coa service")
	}
	coaSvc := coaModel.(*models.SService)
	timeStr, err := coaSvc.Extra.GetString("Expires")
	if err != nil {
		return nil, errors.Wrap(err, "get expires")
	}
	body, err := coaSvc.Extra.Get("Oauth_token")
	if err != nil {
		return nil, errors.Wrap(err, "get oauth token body")
	}
	expiresTime, _ := time.ParseInLocation("2006-01-02 15:04:05", timeStr, time.Local)
	if time.Now().Add(24 * time.Hour).After(expiresTime) {
		oauthToken, err := fetchOauthToken(ctx, body)
		if err != nil {
			return nil, errors.Wrap(err, "get oauth token body")
		}
		//coaSvc.Extra.Set("Expires", jsonutils.NewString(time.Now().Add(time.Duration(oauthToken.Expires) * time.Second).Format("2006-01-02 15:04:05")))
		//coaSvc.Extra.Set("Access_token", jsonutils.NewString(oauthToken.AccessToken))
		//db update
		models.ServiceManager.UpdateExtra(coaSvc, oauthToken.AccessToken, oauthToken.Expires)
	}
	token, err := coaSvc.Extra.GetString("Access_token")
	if err != nil {
		return nil, errors.Wrap(err, "get oauth token")
	}
	emailPrefix := strings.Split(userInfo.Email, "@")[0]

	admins, err := coaSvc.Extra.GetArray("Admins")
	if err != nil {
		return nil, errors.Wrap(err, "get admin whitelist")
	}
	isAdmin := false
	for _, admin := range admins {
		adminStr, _ := admin.GetString()
		//test := jsonutils.Marshal("test")
		//testStr, _ := test.GetString()
		//fmt.Println(test.String())
		//fmt.Println(testStr)
		//if admin.String() == emailPrefix { //不要用String()方法，会把字符串自动加上双引号
		if adminStr == emailPrefix {
			isAdmin = true
			break
		}
	}

	coaInfo, err := fetchCoaUserInfo(ctx, token, strconv.FormatInt(userInfo.StaffId, 10))
	if err != nil {
		return nil, errors.Wrap(err, "fetch coa user")
	}
	coaDepInfo, err := fetchCoaDepartmentsInfo(ctx, token, strconv.FormatInt(coaInfo.DepartmentId, 10))
	if err != nil {
		return nil, errors.Wrap(err, "fetch coa deps")
	}
	deps, err := fetchProjectMetadata()
	//deps, err := coaSvc.Extra.GetArray("Department_id") //deprecate
	if err != nil {
		return nil, errors.Wrap(err, "fetchProjectMetadata department tag")
	}

	projects := make([]string, 0)
	ret := make(map[string][]string)
	idPaths := strings.Split(coaDepInfo.IdPath, "-")

	for _, idStr := range idPaths {
		idInt, _ := strconv.ParseInt(idStr, 10, 64)
		if _, ok := deps[idInt]; ok {
			projects = append(projects, deps[idInt]...)
		}
	}
	if isAdmin {
		projects = append(projects, "system")
		//ret["roles"] = []string{"admin"}
	}
	if len(projects) == 0 {
		return nil, errors.Wrap(fmt.Errorf("Unauthorized"), "Unauthorized")
	}

	//ret := make(map[string][]string)
	ret["name"] = []string{userInfo.Name}
	ret["user_id"] = []string{userInfo.UserID}
	ret["name_en"] = []string{emailPrefix}
	ret["email"] = []string{userInfo.Email}
	ret["mobile"] = []string{userInfo.Mobile}
	ret["avatar"] = []string{userInfo.AvatarURL}
	ret["staff_id"] = []string{strconv.FormatInt(userInfo.StaffId, 10)}
	ret["project"] = projects
	//ret["roles"] = []string{"project_viewer"}
	//if len(projects) == 0 && isAdmin {
	//	ret["project"] = []string{"system"}
	//	ret["roles"] = []string{"admin"}
	//}
	return ret, nil
}
