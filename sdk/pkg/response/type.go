/*
 * @Author: lwnmengjing
 * @Date: 2021/6/8 5:51 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/8 5:51 下午
 */

package response

type Responses interface {
	SetCode(int32)
	SetTraceID(string)
	SetMsg(string)
	SetData(interface{})
	SetSuccess(bool)
	Clone() Responses
}
