package server

import (
	"context"
	"log/slog"

	"github.com/m0od/docu-ui/internal/auth"
	"github.com/m0od/docu-ui/internal/store"
)

type secondFactorResult int

const (
	secondFactorAccepted secondFactorResult = iota
	secondFactorWrong
	secondFactorUsed
)

// problem is the message for a rejected code.
func (result secondFactorResult) problem() string {
	if result == secondFactorUsed {
		return "TOTP code was already used: wait for the next code"
	}
	return wrongTOTPCode
}

// checkSecondFactor accepts a 6-digit code from the TOTP app or, when the phone is lost, one of the
// account's recovery codes. Either works once only.
func checkSecondFactor(ctx context.Context, accounts Store, account store.Account, code string) (secondFactorResult, error) {
	if auth.IsRecoveryCode(code) {
		codeAccepted, err := accounts.UseRecoveryCode(ctx, account.ID, auth.HashRecoveryCode(code))
		if err != nil || !codeAccepted {
			// A used recovery code is gone, so it is simply wrong now.
			return secondFactorWrong, err
		}
		slog.Warn("recovery code used", "user", account.Username)
		return secondFactorAccepted, nil
	}
	codeStep, codeMatched := auth.MatchTOTPStep(account.TOTPSecret, code, now())
	if !codeMatched {
		return secondFactorWrong, nil
	}
	stepAccepted, err := accounts.UseTOTPStep(ctx, account.ID, codeStep)
	if err != nil || !stepAccepted {
		return secondFactorUsed, err
	}
	return secondFactorAccepted, nil
}

// issueRecoveryCodes gives the account a new set of codes, replacing any old ones, and returns
// them in plain text. This is the only time they are shown.
func issueRecoveryCodes(ctx context.Context, accounts Store, accountID int64) ([]string, error) {
	recoveryCodes := auth.NewRecoveryCodes()
	codeHashes := make([]string, len(recoveryCodes))
	for index, recoveryCode := range recoveryCodes {
		codeHashes[index] = auth.HashRecoveryCode(recoveryCode)
	}
	return recoveryCodes, accounts.ReplaceRecoveryCodes(ctx, accountID, codeHashes)
}
