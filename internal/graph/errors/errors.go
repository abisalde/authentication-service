package errors

import (
	"github.com/abisalde/authentication-service/internal/graph/model"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

var (
	RateLimitExceeded = &gqlerror.Error{
		Message: "Too many attempts. Please try again later.",
		Extensions: map[string]interface{}{
			"code": model.ErrorTypeRateLimited,
		},
	}
	AuthenticationRequired = &gqlerror.Error{
		Message: "Access Denied Authentication required.",
		Extensions: map[string]interface{}{
			"code": model.ErrorTypeUnauthenticated,
		},
	}
	UserNotFound = &gqlerror.Error{
		Message: "User not found.",
		Extensions: map[string]interface{}{
			"code": model.ErrorTypeNotFound,
		},
	}
	EmailExists = &gqlerror.Error{
		Message: "User with email address already exist, Please try with a different email address",
		Extensions: map[string]interface{}{
			"code": model.ErrorTypeEmailExists,
		},
	}
	OTPCodeExpire = &gqlerror.Error{
		Message: "Expired verification code",
		Extensions: map[string]interface{}{
			"code": model.ErrorTypeInvalidInput,
		},
	}
	OTPCodeNotValid = &gqlerror.Error{
		Message: "Invalid verification code",
		Extensions: map[string]interface{}{
			"code": model.ErrorTypeForbidden,
		},
	}
	EmailVerificationFailed = &gqlerror.Error{
		Message: "Verification failed, please try again!",
		Extensions: map[string]interface{}{
			"code": "VERIFICATION",
		},
	}
	InvalidCredentialsPassword = &gqlerror.Error{
		Message: "Invalid password provided",
		Extensions: map[string]interface{}{
			"code": model.ErrorTypeWeakPassword,
		},
	}
	InvalidCredentialsEmail = &gqlerror.Error{
		Message: "User with email does not exist",
		Extensions: map[string]interface{}{
			"code": model.ErrorTypeEmail,
		},
	}
	ErrSomethingWentWrong = NewTypedError("Something went wrong! Please try again", model.ErrorTypeBadRequest, map[string]interface{}{})
	InvalidToken          = &gqlerror.Error{
		Message: "Invalid token header",
		Extensions: map[string]interface{}{
			"code": model.ErrorTypeUnauthenticated,
		},
	}
	ExpiredToken = &gqlerror.Error{
		Message: "Expired token",
		Extensions: map[string]interface{}{
			"code": model.ErrorTypeUnauthenticated,
		},
	}
	InvalidTokenType = &gqlerror.Error{
		Message: "Invalid token type",
		Extensions: map[string]interface{}{
			"code": model.ErrorTypeToken,
		},
	}
	JWTSecretNotConfigured = &gqlerror.Error{
		Message: "JWT secret not configured",
		Extensions: map[string]interface{}{
			"code": model.ErrorTypeToken,
		},
	}

	InvalidUserID = &gqlerror.Error{
		Message: "Invalid userID, ID not in range",
		Extensions: map[string]interface{}{
			"code": model.ErrorTypeBadRequest,
		},
	}

	InvalidRefreshTokenValidation = &gqlerror.Error{
		Message: "Unable to validate refresh token, try again or Logout and Login again",
		Extensions: map[string]interface{}{
			"code": model.ErrorTypeRefreshToken,
		},
	}

	AccessTokenGeneration = &gqlerror.Error{
		Message: "There's an error generating token, please try again",
		Extensions: map[string]interface{}{
			"code": model.ErrorTypeRefreshToken,
		},
	}
)
