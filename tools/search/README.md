## Search库

|type|描述|query示例|
|:---|:---|:---|
|exact/iexact|等于|status=1|
|contains/icontanins|包含|name=n|
|gt/gte|大于/大于等于|age=18|
|lt/lte|小于/小于等于|age=18|
|startswith/istartswith|以…起始|content=hell|
|endswith/iendswith|以…结束|content=world|
|in|in查询|status[]=0&status[]=1|
|isnull|isnull查询|startTime=1|
|json_contains|JSON 数组精确包含值（MySQL `JSON_CONTAINS` / Postgres `@>`）|agentCode=ai-001|
|json_contains_or_null|JSON 数组三态白名单：NULL/空数组=全部命中；非空=按值匹配|agentCode=ai-001|
|order|排序|sort=asc/sort=desc|

e.g.
```
type ApplicationQuery struct {
	Id       string    `search:"type:icontains;column:id;table:receipt" form:"id"`
	Domain   string    `search:"type:icontains;column:domain;table:receipt" form:"domain"`
	Version  string    `search:"type:exact;column:version;table:receipt" form:"version"`
	Status   []int     `search:"type:in;column:status;table:receipt" form:"status"`
	Start    time.Time `search:"type:gte;column:created_at;table:receipt" form:"start"`
	End      time.Time `search:"type:lte;column:created_at;table:receipt" form:"end"`
	TestJoin `search:"type:left;on:id:receipt_id;table:receipt_goods;join:receipts"`
	ApplicationOrder
}
type ApplicationOrder struct {
	IdOrder string `search:"type:order;column:id;table:receipt" form"id_order"`
}

type TestJoin struct {
	PaymentAccount string `search:"type:icontains;column:payment_account;table:receipts" form:"payment_account"`
}
```
