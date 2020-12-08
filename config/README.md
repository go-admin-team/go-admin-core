###

测试用例：
```
import (
	"fmt"
	"testing"
	
	"github.com/go-admin-team/go-admin-core/config"
	"github.com/go-admin-team/go-admin-core/config/source/file"
)

func TestApp(t *testing.T)  {
	c, err := config.NewConfig()
	if err != nil {
		t.Error(err)
	}
	err = c.Load(file.NewSource(file.WithPath("config/settings.yml")))
	if err != nil {
		t.Error(err)
	}
	fmt.Println(c.Map())
}
```
