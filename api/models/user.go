package models

import (
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

type LoginUser struct {
	Username string `json:"username" valid:"required~please provide your username"`
	Password string `json:"password" valid:"required~please provide your password"`
}

func (lu *LoginUser) Validate() error {
	return util.Validate(lu)
}

type RegisterUser struct {
	FirstName        string `json:"first_name" valid:"required~please provide a first name"`
	LastName         string `json:"last_name" valid:"required~please provide a last name"`
	Email            string `json:"email" valid:"required~please provide an email,email"`
	Password         string `json:"password" valid:"required~please provide a password"`
	OrganisationName string `json:"org_name" valid:"required~please provide an organisation name"`
}

func (ru *RegisterUser) Validate() error {
	return util.Validate(ru)
}

type UpdateUser struct {
	FirstName string `json:"first_name" valid:"required~please provide a first name"`
	LastName  string `json:"last_name" valid:"required~please provide a last name"`
	Email     string `json:"email" valid:"required~please provide an email,email"`
}

func (u *UpdateUser) Validate() error {
	return util.Validate(u)
}

type UpdatePassword struct {
	CurrentPassword      string `json:"current_password" valid:"required~please provide the current password"`
	Password             string `json:"password" valid:"required~please provide the password field"`
	PasswordConfirmation string `json:"password_confirmation" valid:"required~please provide the password confirmation field"`
}

func (u *UpdatePassword) Validate() error {
	return util.Validate(u)
}

type UserExists struct {
	Email string `json:"email" valid:"required~please provide an email,email"`
}

func (ue *UserExists) Validate() error {
	return util.Validate(ue)
}

type User struct {
	FirstName string    `json:"first_name" valid:"required~please provide a first name"`
	LastName  string    `json:"last_name" valid:"required~please provide a last name"`
	Email     string    `json:"email" valid:"required~please provide an email,email"`
	Password  string    `json:"password" valid:"required~please provide a password"`
	Role      auth.Role `json:"role" bson:"role"`
}

type ForgotPassword struct {
	Email string `json:"email" valid:"required~please provide an email,email"`
}

func (fp *ForgotPassword) Validate() error {
	return util.Validate(fp)
}

type ResetPassword struct {
	Password             string `json:"password" valid:"required~please provide the password field"`
	PasswordConfirmation string `json:"password_confirmation" valid:"required~please provide the password confirmation field"`
}

func (rp *ResetPassword) Validate() error {
	return util.Validate(rp)
}

type UserResponse struct {
	*datastore.User
}

type Token struct {
	AccessToken  string `json:"access_token" valid:"required~please provide an access token"`
	RefreshToken string `json:"refresh_token" valid:"required~please provide a refresh token"`
}

func (t *Token) Validate() error {
	return util.Validate(t)
}

type LoginUserResponse struct {
	*datastore.User
	Token Token `json:"token"`
}
