package api

import (
	"fmt"
	"testing"

	"github.com/gin-gonic/gin/binding"
	. "github.com/smartystreets/goconvey/convey"
)

// uriJSONReq 复现根因 DTO：同含 uri（id）与 json（recallMode required）。
type uriJSONReq struct {
	Id         int64  `uri:"id" json:"-"`
	RecallMode string `json:"recallMode" binding:"required"`
	Weight     int    `json:"weight"`
}

type pureJSONReq struct {
	Name string `json:"name" binding:"required"`
}

type pureURIReq struct {
	Id int64 `uri:"id"`
}

type formURIReq struct {
	Id   int64  `uri:"id"`
	Page string `form:"page"`
}

// TestGetBindingForGinDeterministicOrder 锁死 binding 返回顺序：
// body 类（JSON/Form 等）恒在前，uri（nil）恒在最后。
// 旧实现用 map[uint8]binding + for range 返回，遍历顺序随机，此用例会间歇性失败；
// 修复后跨多次调用顺序恒定。每个用例额外用全新类型（无缓存）跑足够多次以触发 map 随机。
func TestGetBindingForGinDeterministicOrder(t *testing.T) {
	Convey("uri+json 混合：JSON 必在前、uri(nil) 必在最后", t, func() {
		for i := 0; i < 500; i++ {
			list := constructor.GetBindingForGin(&uriJSONReq{})
			So(len(list), ShouldEqual, 2)
			So(list[0], ShouldEqual, binding.JSON)
			So(list[1], ShouldBeNil)
		}
	})

	Convey("纯 json：仅返回 [JSON]，无 nil 尾项", t, func() {
		list := constructor.GetBindingForGin(&pureJSONReq{})
		So(len(list), ShouldEqual, 1)
		So(list[0], ShouldEqual, binding.JSON)
	})

	Convey("纯 uri：仅返回 [nil]", t, func() {
		list := constructor.GetBindingForGin(&pureURIReq{})
		So(len(list), ShouldEqual, 1)
		So(list[0], ShouldBeNil)
	})

	Convey("form+uri 混合：Form 在前、uri(nil) 在最后", t, func() {
		for i := 0; i < 500; i++ {
			list := constructor.GetBindingForGin(&formURIReq{})
			So(len(list), ShouldEqual, 2)
			So(list[0], ShouldEqual, binding.Form)
			So(list[1], ShouldBeNil)
		}
	})
}

type Pagination struct {
	PageIndex int `form:"pageIndex"`
	PageSize  int `form:"pageSize"`
}

type SysUserSearch struct {
	Pagination `search:"-"`
	UserId     int    `form:"userId" search:"type:exact;column:user_id;table:sys_user" comment:"用户ID"`
	Username   string `form:"username" search:"type:contains;column:username;table:sys_user" comment:"用户名"`
	NickName   string `form:"nickName" search:"type:contains;column:nick_name;table:sys_user" comment:"昵称"`
	Phone      string `form:"phone" search:"type:contains;column:phone;table:sys_user" comment:"手机号"`
	RoleId     string `form:"roleId" search:"type:exact;column:role_id;table:sys_user" comment:"角色ID"`
	Sex        string `form:"sex" search:"type:exact;column:sex;table:sys_user" comment:"性别"`
	Email      string `form:"email" search:"type:contains;column:email;table:sys_user" comment:"邮箱"`
	DeptId     string `form:"deptId" search:"type:exact;column:dept_id;table:sys_user" comment:"部门"`
	PostId     string `form:"postId" search:"type:exact;column:post_id;table:sys_user" comment:"岗位"`
	Status     string `form:"status" search:"type:exact;column:status;table:sys_user" comment:"状态"`
	SysUserOrder
}

type SysUserOrder struct {
	UserIdOrder    string `search:"type:order;column:user_id;table:sys_user" form:"userIdOrder"`
	UsernameOrder  string `search:"type:order;column:username;table:sys_user" form:"usernameOrder"`
	StatusOrder    string `search:"type:order;column:status;table:sys_user" form:"statusOrder"`
	CreatedAtOrder string `search:"type:order;column:created_at;table:sys_user" form:"createdAtOrder"`
}

func TestResolve(t *testing.T) {
	// Only pass t into top-level Convey calls
	Convey("Given some integer with a starting value", t, func() {

		d := &SysUserSearch{}

		list := constructor.GetBindingForGin(d)
		for _, binding := range list {
			fmt.Printf("%v /n",binding)
		}

	})
}
