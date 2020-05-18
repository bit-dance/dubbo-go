/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package common

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"math"
	"net"
	"net/url"
	"strconv"
	"strings"
)

import (
	gxset "github.com/dubbogo/gost/container/set"
	"github.com/jinzhu/copier"
	perrors "github.com/pkg/errors"
	"github.com/satori/go.uuid"
)

import (
	"github.com/apache/dubbo-go/common/constant"
)

// ///////////////////////////////
// dubbo role type
// ///////////////////////////////

// role constant
const (
	// CONSUMER ...
	CONSUMER = iota
	// CONFIGURATOR ...
	CONFIGURATOR
	// ROUTER ...
	ROUTER
	// PROVIDER ...
	PROVIDER
)

var (
	// DubboNodes ...
	DubboNodes = [...]string{"consumers", "configurators", "routers", "providers"}
	// DubboRole Dubbo service role
	DubboRole = [...]string{"consumer", "", "routers", "provider"}
)

// RoleType ...
type RoleType int

func (t RoleType) String() string {
	return DubboNodes[t]
}

// Role ...
func (t RoleType) Role() string {
	return DubboRole[t]
}

type baseUrl struct {
	Protocol string
	Location string // ip+port
	Ip       string
	Port     string
	params       url.Values
	PrimitiveURL string
}

// URL is not thread-safe.
// we fail to define this struct to be immutable object.
// but, those method which will update the URL, including SetParam, SetParams
// are only allowed to be invoked in creating URL instance
// Please keep in mind that this struct is immutable after it has been created and initialized.
type URL struct {
	baseUrl
	Path     string // like  /com.ikurento.dubbo.UserProvider3
	Username string
	Password string
	Methods  []string
	// special for registry
	SubURL *URL
}

type option func(*URL)

// WithUsername ...
func WithUsername(username string) option {
	return func(url *URL) {
		url.Username = username
	}
}

// WithPassword ...
func WithPassword(pwd string) option {
	return func(url *URL) {
		url.Password = pwd
	}
}

// WithMethods ...
func WithMethods(methods []string) option {
	return func(url *URL) {
		url.Methods = methods
	}
}

// WithParams ...
func WithParams(params url.Values) option {
	return func(url *URL) {
		url.params = params
	}
}

// WithParamsValue ...
func WithParamsValue(key, val string) option {
	return func(url *URL) {
		url.SetParam(key, val)
	}
}

// WithProtocol ...
func WithProtocol(proto string) option {
	return func(url *URL) {
		url.Protocol = proto
	}
}

// WithIp ...
func WithIp(ip string) option {
	return func(url *URL) {
		url.Ip = ip
	}
}

// WithPort ...
func WithPort(port string) option {
	return func(url *URL) {
		url.Port = port
	}
}

// WithPath ...
func WithPath(path string) option {
	return func(url *URL) {
		url.Path = "/" + strings.TrimPrefix(path, "/")
	}
}

// WithLocation ...
func WithLocation(location string) option {
	return func(url *URL) {
		url.Location = location
	}
}

// WithToken ...
func WithToken(token string) option {
	return func(url *URL) {
		if len(token) > 0 {
			value := token
			if strings.ToLower(token) == "true" || strings.ToLower(token) == "default" {
				value = uuid.NewV4().String()
			}
			url.SetParam(constant.TOKEN_KEY, value)
		}
	}
}

// NewURLWithOptions ...
func NewURLWithOptions(opts ...option) *URL {
	url := &URL{}
	for _, opt := range opts {
		opt(url)
	}
	url.Location = url.Ip + ":" + url.Port
	return url
}

// NewURL will create a new url
// the urlString should not be empty
func NewURL(urlString string, opts ...option) (URL, error) {
	var (
		err          error
		rawUrlString string
		serviceUrl   *url.URL
		s            = URL{baseUrl: baseUrl{}}
	)

	// new a null instance
	if urlString == "" {
		return s, nil
	}

	rawUrlString, err = url.QueryUnescape(urlString)
	if err != nil {
		return s, perrors.Errorf("url.QueryUnescape(%s),  error{%v}", urlString, err)
	}

	// rawUrlString = "//" + rawUrlString
	if strings.Index(rawUrlString, "//") < 0 {
		t := URL{baseUrl: baseUrl{}}
		for _, opt := range opts {
			opt(&t)
		}
		rawUrlString = t.Protocol + "://" + rawUrlString
	}
	serviceUrl, err = url.Parse(rawUrlString)
	if err != nil {
		return s, perrors.Errorf("url.Parse(url string{%s}),  error{%v}", rawUrlString, err)
	}

	s.params, err = url.ParseQuery(serviceUrl.RawQuery)
	if err != nil {
		return s, perrors.Errorf("url.ParseQuery(raw url string{%s}),  error{%v}", serviceUrl.RawQuery, err)
	}

	s.PrimitiveURL = urlString
	s.Protocol = serviceUrl.Scheme
	s.Username = serviceUrl.User.Username()
	s.Password, _ = serviceUrl.User.Password()
	s.Location = serviceUrl.Host
	s.Path = serviceUrl.Path
	if strings.Contains(s.Location, ":") {
		s.Ip, s.Port, err = net.SplitHostPort(s.Location)
		if err != nil {
			return s, perrors.Errorf("net.SplitHostPort(url.Host{%s}), error{%v}", s.Location, err)
		}
	}
	for _, opt := range opts {
		opt(&s)
	}
	return s, nil
}

// URLEqual judge @url and @c is equal or not.
func (c URL) URLEqual(url URL) bool {
	c.Ip = ""
	c.Port = ""
	url.Ip = ""
	url.Port = ""
	cGroup := c.GetParam(constant.GROUP_KEY, "")
	urlGroup := url.GetParam(constant.GROUP_KEY, "")
	cKey := c.Key()
	urlKey := url.Key()

	if cGroup == constant.ANY_VALUE {
		cKey = strings.Replace(cKey, "group=*", "group="+urlGroup, 1)
	} else if urlGroup == constant.ANY_VALUE {
		urlKey = strings.Replace(urlKey, "group=*", "group="+cGroup, 1)
	}

	// 1. protocol, username, password, ip, port, service name, group, version should be equal
	if cKey != urlKey {
		return false
	}

	// 2. if url contains enabled key, should be true, or *
	if url.GetParam(constant.ENABLED_KEY, "true") != "true" && url.GetParam(constant.ENABLED_KEY, "") != constant.ANY_VALUE {
		return false
	}

	// TODO :may need add interface key any value condition
	return isMatchCategory(url.GetParam(constant.CATEGORY_KEY, constant.DEFAULT_CATEGORY), c.GetParam(constant.CATEGORY_KEY, constant.DEFAULT_CATEGORY))
}

func isMatchCategory(category1 string, category2 string) bool {
	if len(category2) == 0 {
		return category1 == constant.DEFAULT_CATEGORY
	} else if strings.Contains(category2, constant.ANY_VALUE) {
		return true
	} else if strings.Contains(category2, constant.REMOVE_VALUE_PREFIX) {
		return !strings.Contains(category2, constant.REMOVE_VALUE_PREFIX+category1)
	} else {
		return strings.Contains(category2, category1)
	}
}

func (c URL) String() string {
	var buildString string
	if len(c.Username) == 0 && len(c.Password) == 0 {
		buildString = fmt.Sprintf(
			"%s://%s:%s%s?",
			c.Protocol, c.Ip, c.Port, c.Path)
	} else {
		buildString = fmt.Sprintf(
			"%s://%s:%s@%s:%s%s?",
			c.Protocol, c.Username, c.Password, c.Ip, c.Port, c.Path)
	}
	buildString += c.params.Encode()
	return buildString
}

// Key ...
func (c URL) Key() string {
	buildString := fmt.Sprintf(
		"%s://%s:%s@%s:%s/?interface=%s&group=%s&version=%s",
		c.Protocol, c.Username, c.Password, c.Ip, c.Port, c.Service(), c.GetParam(constant.GROUP_KEY, ""), c.GetParam(constant.VERSION_KEY, ""))
	return buildString
}

// ServiceKey get a unique key of a service.
func (c URL) ServiceKey() string {
	intf := c.GetParam(constant.INTERFACE_KEY, strings.TrimPrefix(c.Path, "/"))
	if intf == "" {
		return ""
	}
	buf := &bytes.Buffer{}
	group := c.GetParam(constant.GROUP_KEY, "")
	if group != "" {
		buf.WriteString(group)
		buf.WriteString("/")
	}

	buf.WriteString(intf)

	version := c.GetParam(constant.VERSION_KEY, "")
	if version != "" && version != "0.0.0" {
		buf.WriteString(":")
		buf.WriteString(version)
	}

	return buf.String()
}

// ColonSeparatedKey
// The format is "{interface}:[version]:[group]"
func (c *URL) ColonSeparatedKey() string {
	intf := c.GetParam(constant.INTERFACE_KEY, strings.TrimPrefix(c.Path, "/"))
	if intf == "" {
		return ""
	}
	buf := &bytes.Buffer{}
	buf.WriteString(intf)
	buf.WriteString(":")
	version := c.GetParam(constant.VERSION_KEY, "")
	if version != "" && version != "0.0.0" {
		buf.WriteString(version)
	}
	group := c.GetParam(constant.GROUP_KEY, "")
	buf.WriteString(":")
	if group != "" {
		buf.WriteString(group)
	}
	return buf.String()
}

// EncodedServiceKey ...
func (c *URL) EncodedServiceKey() string {
	serviceKey := c.ServiceKey()
	return strings.Replace(serviceKey, "/", "*", 1)
}

// Service ...
func (c URL) Service() string {
	service := c.GetParam(constant.INTERFACE_KEY, strings.TrimPrefix(c.Path, "/"))
	if service != "" {
		return service
	} else if c.SubURL != nil {
		service = c.GetParam(constant.INTERFACE_KEY, strings.TrimPrefix(c.Path, "/"))
		if service != "" { // if url.path is "" then return suburl's path, special for registry url
			return service
		}
	}
	return ""
}

// AddParam will add the key-value pair
// Not thread-safe
// think twice before using it.
func (c *URL) AddParam(key string, value string) {
	c.params.Add(key, value)
}

// SetParam will put the key-value pair into url
// it's not thread safe.
// think twice before you want to use this method
// usually it should only be invoked when you want to initialized an url
func (c *URL) SetParam(key string, value string) {
	c.params.Set(key, value)
}

// RangeParams will iterate the params
// it's not thread-safe
func (c *URL) RangeParams(f func(key, value string) bool) {
	for k, v := range c.params {
		if !f(k, v[0]) {
			break
		}
	}
}

// GetParam ...
func (c URL) GetParam(s string, d string) string {
	// c.paramsLock.RLock()
	// defer c.paramsLock.RUnlock()
	r := c.params.Get(s)
	if len(r) == 0 {
		r = d
	}
	return r
}

// GetParams ...
func (c URL) GetParams() url.Values {
	return c.params
}

// GetParamAndDecoded ...
func (c URL) GetParamAndDecoded(key string) (string, error) {
	// c.paramsLock.RLock()
	// defer c.paramsLock.RUnlock()
	ruleDec, err := base64.URLEncoding.DecodeString(c.GetParam(key, ""))
	value := string(ruleDec)
	return value, err
}

// GetRawParam ...
func (c URL) GetRawParam(key string) string {
	switch key {
	case "protocol":
		return c.Protocol
	case "username":
		return c.Username
	case "host":
		return strings.Split(c.Location, ":")[0]
	case "password":
		return c.Password
	case "port":
		return c.Port
	case "path":
		return c.Path
	default:
		return c.GetParam(key, "")
	}
}

// GetParamBool ...
func (c URL) GetParamBool(s string, d bool) bool {
	r, err := strconv.ParseBool(c.GetParam(s, ""))
	if err != nil {
		return d
	}
	return r
}

// GetParamInt ...
func (c URL) GetParamInt(s string, d int64) int64 {
	r, err := strconv.Atoi(c.GetParam(s, ""))
	if r == 0 || err != nil {
		return d
	}
	return int64(r)
}

// GetMethodParamInt ...
func (c URL) GetMethodParamInt(method string, key string, d int64) int64 {
	r, err := strconv.Atoi(c.GetParam("methods."+method+"."+key, ""))
	if r == 0 || err != nil {
		return d
	}
	return int64(r)
}

// GetMethodParamInt64 ...
func (c URL) GetMethodParamInt64(method string, key string, d int64) int64 {
	r := c.GetMethodParamInt(method, key, math.MinInt64)
	if r == math.MinInt64 {
		return c.GetParamInt(key, d)
	}
	return r
}

// GetMethodParam ...
func (c URL) GetMethodParam(method string, key string, d string) string {
	r := c.GetParam("methods."+method+"."+key, "")
	if r == "" {
		r = d
	}
	return r
}

// GetMethodParamBool ...
func (c URL) GetMethodParamBool(method string, key string, d bool) bool {
	r := c.GetParamBool("methods."+method+"."+key, d)
	return r
}

// SetParams will put all key-value pair into url.
// 1. if there already has same key, the value will be override
// 2. it's not thread safe
// 3. think twice when you want to invoke this method
func (c *URL) SetParams(m url.Values) {
	for k := range m {
		c.SetParam(k, m.Get(k))
	}
}

// ToMap transfer URL to Map
func (c URL) ToMap() map[string]string {
	paramsMap := make(map[string]string)

	c.RangeParams(func(key, value string) bool {
		paramsMap[key] = value
		return true
	})

	if c.Protocol != "" {
		paramsMap["protocol"] = c.Protocol
	}
	if c.Username != "" {
		paramsMap["username"] = c.Username
	}
	if c.Password != "" {
		paramsMap["password"] = c.Password
	}
	if c.Location != "" {
		paramsMap["host"] = strings.Split(c.Location, ":")[0]
		var port string
		if strings.Contains(c.Location, ":") {
			port = strings.Split(c.Location, ":")[1]
		} else {
			port = "0"
		}
		paramsMap["port"] = port
	}
	if c.Protocol != "" {
		paramsMap["protocol"] = c.Protocol
	}
	if c.Path != "" {
		paramsMap["path"] = c.Path
	}
	if len(paramsMap) == 0 {
		return nil
	}
	return paramsMap
}

// configuration  > reference config >service config
//  in this function we should merge the reference local url config into the service url from registry.
// TODO configuration merge, in the future , the configuration center's config should merge too.

// MergeUrl will merge those two url
// the result is based on serviceUrl, and the key which si only contained in referenceUrl
// will be added into result.
// for example, if serviceUrl contains params (a1->v1, b1->v2) and referenceUrl contains params(a2->v3, b1 -> v4)
// the params of result will be (a1->v1, b1->v2, a2->v3).
// You should notice that the value of b1 is v2, not v4.
// due to URL is not thread-safe, so this method is not thread-safe
func MergeUrl(serviceUrl *URL, referenceUrl *URL) *URL {
	mergedUrl := serviceUrl.Clone()

	// iterator the referenceUrl if serviceUrl not have the key ,merge in
	referenceUrl.RangeParams(func(key, value string) bool {
		if v := mergedUrl.GetParam(key, ""); len(v) == 0 {
			mergedUrl.SetParam(key, value)
		}
		return true
	})
	// loadBalance,cluster,retries strategy config
	methodConfigMergeFcn := mergeNormalParam(mergedUrl, referenceUrl, []string{constant.LOADBALANCE_KEY, constant.CLUSTER_KEY, constant.RETRIES_KEY, constant.TIMEOUT_KEY})

	// remote timestamp
	if v := serviceUrl.GetParam(constant.TIMESTAMP_KEY, ""); len(v) > 0 {
		mergedUrl.SetParam(constant.REMOTE_TIMESTAMP_KEY, v)
		mergedUrl.SetParam(constant.TIMESTAMP_KEY, referenceUrl.GetParam(constant.TIMESTAMP_KEY, ""))
	}

	// finally execute methodConfigMergeFcn
	for _, method := range referenceUrl.Methods {
		for _, fcn := range methodConfigMergeFcn {
			fcn("methods." + method)
		}
	}

	return mergedUrl
}

// Clone will copy the url
func (c *URL) Clone() *URL {
	newUrl := &URL{}
	copier.Copy(newUrl, c)
	newUrl.params = url.Values{}
	c.RangeParams(func(key, value string) bool {
		newUrl.SetParam(key, value)
		return true
	})
	return newUrl
}

func (c *URL) CloneExceptParams(excludeParams *gxset.HashSet) *URL {
	newUrl := &URL{}
	copier.Copy(newUrl, c)
	newUrl.params = url.Values{}
	c.RangeParams(func(key, value string) bool {
		if !excludeParams.Contains(key) {
			newUrl.SetParam(key, value)
		}
		return true
	})
	return newUrl
}

// Copy url based on the reserved parameters' keys.
func (c *URL) CloneWithParams(reserveParams []string) *URL {
	params := url.Values{}
	for _, reserveParam := range reserveParams {
		v := c.GetParam(reserveParam, "")
		if len(v) != 0 {
			params.Set(reserveParam, v)
		}
	}

	return NewURLWithOptions(
		WithProtocol(c.Protocol),
		WithUsername(c.Username),
		WithPassword(c.Password),
		WithIp(c.Ip),
		WithPort(c.Port),
		WithPath(c.Path),
		WithMethods(c.Methods),
		WithParams(params),
	)
}

func mergeNormalParam(mergedUrl *URL, referenceUrl *URL, paramKeys []string) []func(method string) {
	methodConfigMergeFcn := make([]func(method string), 0, len(paramKeys))
	for _, paramKey := range paramKeys {
		if v := referenceUrl.GetParam(paramKey, ""); len(v) > 0 {
			mergedUrl.SetParam(paramKey, v)
		}
		methodConfigMergeFcn = append(methodConfigMergeFcn, func(method string) {
			if v := referenceUrl.GetParam(method+"."+paramKey, ""); len(v) > 0 {
				mergedUrl.SetParam(method+"."+paramKey, v)
			}
		})
	}
	return methodConfigMergeFcn
}
