package helper

import (
	"context"
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"github.com/sirupsen/logrus"
	"veverse-api/model"
	"veverse-api/reflect"
)

func CanLikable(ctx context.Context, requester *sm.User, entity model.Entity) bool {

	isOwner, canView, _, _, err := model.EntityAccessible(ctx, requester.Id, *entity.Id)
	if err != nil {
		logrus.Errorf("failed to get check accessible %s @ %s: %v", model.AccessibleSingular, reflect.FunctionName(), err)
	}

	switch {
	case requester.IsAdmin || requester.IsInternal:
		return true
	case *entity.Public:
		return true
	case isOwner || canView:
		return true
	}

	return false
}
