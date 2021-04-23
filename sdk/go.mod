module github.com/go-admin-team/go-admin-core/sdk

go 1.14

require (
	github.com/bsm/redislock v0.5.0
	github.com/casbin/casbin/v2 v2.24.0
	github.com/chanxuehong/wechat v0.0.0-20201110083048-0180211b69fd
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/gin-gonic/gin v1.6.3
	github.com/go-admin-team/go-admin-core 40bb8878c4c00555fe2c38f16914712eab4d41ed
	github.com/go-admin-team/gorm-adapter/v3 v3.2.1-0.20210310135230-1608cc35b95b
	github.com/go-redis/redis/v7 v7.4.0
	github.com/google/uuid v1.1.2
	github.com/gorilla/websocket v1.4.2
	github.com/mojocn/base64Captcha v1.3.1
	github.com/robfig/cron/v3 v3.0.1
	github.com/robinjoseph08/redisqueue/v2 v2.1.0
	github.com/shamsher31/goimgext v1.0.0
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	gorm.io/gorm v1.21.6
)

replace github.com/go-admin-team/go-admin-core => ../
