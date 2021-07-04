package api

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

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

		d := SysUserSearch{}

		list := constructor.GetBindingForGin(d)
		for _, binding := range list {
			fmt.Printf("%v /n",binding)
		}

	})
}
