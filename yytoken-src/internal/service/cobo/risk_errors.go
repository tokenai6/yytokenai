package cobo

import (
	"errors"
	"fmt"
)

var (
	errWithdrawBalanceInsufficient     = errors.New("withdraw balance insufficient")
	errWithdrawRiskNewUserLimit        = errors.New("withdraw risk new user limit")
	errWithdrawRisk24hFrequency        = errors.New("withdraw risk 24h frequency")
	errWithdrawRiskTotalLimit          = errors.New("withdraw risk total limit")
	errWithdrawRiskGiftNodeUnqualified = errors.New("withdraw risk gift node unqualified")
	errWithdrawExceedNodeQuota         = errors.New("withdraw exceed node quota")
	errWithdrawTaxConfigMissing        = errors.New("withdraw tax config missing")
)

type riskWrappedError struct {
	cause error
	msg   string
}

func (e *riskWrappedError) Error() string {
	return e.msg
}

func (e *riskWrappedError) Unwrap() error {
	return e.cause
}

func wrapRiskError(cause error, format string, args ...interface{}) error {
	return &riskWrappedError{cause: cause, msg: fmt.Sprintf(format, args...)}
}
