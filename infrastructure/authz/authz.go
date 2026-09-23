package authz

import (
	"errors"

	"github.com/google/uuid"
)

var ErrForbidden = errors.New("forbidden")

func RequireOwnership(actorID, ownerID uuid.UUID) error {
	if actorID == uuid.Nil || ownerID == uuid.Nil || actorID != ownerID {
		return ErrForbidden
	}
	return nil
}

func RequireRole(role string, allowed ...string) error {
	for _, item := range allowed {
		if role == item {
			return nil
		}
	}
	return ErrForbidden
}
