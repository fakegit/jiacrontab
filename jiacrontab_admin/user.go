package admin

import (
	"fmt"
	"jiacrontab/models"
	"jiacrontab/pkg/proto"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/kataras/iris"
)

type CustomerClaims struct {
	jwt.StandardClaims
	UserID   uint
	Mail     string
	Username string
	GroupID  uint
	Root     bool
}

func login(c iris.Context) {
	var (
		err            error
		ctx            = wrapCtx(c)
		reqBody        loginReqParams
		user           models.User
		customerClaims CustomerClaims
	)

	if err = reqBody.verify(ctx); err != nil {
		ctx.respError(proto.Code_Error, err.Error(), nil)
		return
	}
	if !user.Verify(reqBody.Username, reqBody.Passwd) {
		ctx.respError(proto.Code_FailedAuth, "帐号或密码不正确", nil)
		return
	}

	customerClaims.ExpiresAt = cfg.Jwt.Expires + time.Now().Unix()
	customerClaims.Username = reqBody.Username
	customerClaims.UserID = user.ID
	customerClaims.Mail = user.Mail
	customerClaims.GroupID = user.GroupID
	customerClaims.Root = user.Root

	if reqBody.Remember {
		customerClaims.ExpiresAt = time.Now().Add(24 * 30 * time.Hour).Unix()
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, customerClaims).SignedString([]byte(cfg.Jwt.SigningKey))

	if err != nil {
		ctx.respError(proto.Code_FailedAuth, "无法生成访问凭证", nil)
		return
	}

	ctx.respSucc("", token)
}

func getRelationEvent(c iris.Context) {
	var (
		ctx            = wrapCtx(c)
		err            error
		customerClaims CustomerClaims
		reqBody        readMoreReqParams
		events         []models.Event
	)

	if err = reqBody.verify(ctx); err != nil {
		ctx.respError(proto.Code_Error, err.Error(), nil)
		return
	}

	if customerClaims, err = ctx.getClaimsFromToken(); err != nil {
		ctx.respError(proto.Code_Error, "无法获得token信息", err)
		return
	}

	err = models.DB().Where("user_id=?", customerClaims.UserID).Order(fmt.Sprintf("create_at %s", reqBody.Orderby)).
		Find(&events).Error

	if err != nil {
		ctx.respError(proto.Code_Error, "暂无数据", err)
		return
	}

	ctx.respSucc("", map[string]interface{}{
		"list":     events,
		"pagesize": reqBody.Pagesize,
	})
}

func auditJob(c iris.Context) {
	var (
		ctx            = wrapCtx(c)
		err            error
		customerClaims CustomerClaims
		reqBody        auditJobReqParams
		node           models.Node
		reply          bool
	)

	if err = reqBody.verify(ctx); err != nil {
		ctx.respError(proto.Code_Error, err.Error(), nil)
		return
	}

	if customerClaims, err = ctx.getClaimsFromToken(); err != nil {
		ctx.respError(proto.Code_Error, "无法获得token信息", err)
		return
	}

	if customerClaims.GroupID != 0 {
		if customerClaims.Root == false {
			ctx.respError(proto.Code_Error, "无权操作", nil)
			return
		}

		node.GroupID = customerClaims.GroupID
		node.Addr = reqBody.Addr
		if node.Exists() == false {
			ctx.respError(proto.Code_Error, "无权操作", nil)
			return
		}
	}

	if reqBody.JobType == "crontab" {
		if err = rpcCall(reqBody.Addr, "CrontabJob.AuditJob", proto.AuditJobArgs{
			JobID: reqBody.JobID,
		}, &reply); err != nil {
			ctx.respError(proto.Code_Error, err.Error(), nil)
			return
		}
		ctx.pubEvent(event_AuditCrontabJob, reqBody.Addr, reqBody)
	} else {
		if err = rpcCall(reqBody.Addr, "DaemonJob.AuditJob", proto.AuditJobArgs{
			JobID: reqBody.JobID,
		}, &reply); err != nil {
			ctx.respError(proto.Code_Error, err.Error(), nil)
			return
		}
		ctx.pubEvent(event_AuditDaemonJob, reqBody.Addr, reqBody)
	}

	ctx.respSucc("", reply)

}

func signUp(c iris.Context) {
	var (
		err     error
		ctx     = wrapCtx(c)
		user    models.User
		reqBody userReqParams
	)

	if err = reqBody.verify(ctx); err != nil {
		ctx.respError(proto.Code_Error, err.Error(), nil)
		return
	}
	user.Username = reqBody.Username
	user.Passwd = reqBody.Passwd
	user.GroupID = reqBody.GroupID
	user.Root = reqBody.Root
	user.Mail = reqBody.Mail

	if err = user.Create(); err != nil {
		ctx.respError(proto.Code_Error, err.Error(), nil)
		return
	}

	ctx.pubEvent(event_SignUpUser, "", reqBody)
	ctx.respSucc("", true)
}