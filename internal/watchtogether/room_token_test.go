package watchtogether

import (
	"testing"
	"time"
)

func TestRoomTokenServiceMintAndValidate(t *testing.T) {
	service := NewRoomTokenService("secret", time.Hour)
	token, _, err := service.Mint(RoomTokenClaims{
		RoomID:    "room-1",
		UserID:    7,
		ProfileID: "profile-1",
	})
	if err != nil {
		t.Fatalf("Mint() error = %v", err)
	}

	claims, err := service.Validate(token)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if claims.RoomID != "room-1" || claims.UserID != 7 || claims.ProfileID != "profile-1" {
		t.Fatalf("Validate() claims = %+v", claims)
	}
}
