package authentication

import (
	"encoding/json"

	"github.com/ZupIT/horusec-devkit/pkg/enums/ozzovalidation"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"

	"github.com/ZupIT/horusec-devkit/pkg/utils/crypto"
)

type LoginCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (l *LoginCredentials) Validate() error {
	return validation.ValidateStruct(l,
		validation.Field(&l.Username, validation.Required,
			validation.Length(ozzovalidation.Length0, ozzovalidation.Length255)),
		validation.Field(&l.Password, validation.Required,
			validation.Length(ozzovalidation.Length0, ozzovalidation.Length255)),
	)
}

func (l *LoginCredentials) CheckInvalidPassword(hash string) bool {
	return !crypto.CheckPasswordHashBcrypt(l.Password, hash)
}

func (l *LoginCredentials) IsInvalidUsernameEmail() bool {
	return validation.Validate(&l.Username, is.EmailFormat) != nil
}

func (l *LoginCredentials) ToBytes() []byte {
	bytes, _ := json.Marshal(l)
	return bytes
}
