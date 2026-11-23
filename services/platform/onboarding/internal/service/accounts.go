package service

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	onboardingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1"
	"github.com/khayaai/forge-mutual/pkg/licence"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/accounts"
)

// AccountOpener opens further currency accounts (internal/accounts).
type AccountOpener interface {
	Open(ctx context.Context, party, asset, actor string) (accounts.Account, error)
}

// WithAccounts sets how currency accounts are opened. Without one, OpenCurrencyAccount is unavailable.
func (s *Service) WithAccounts(o AccountOpener) *Service {
	s.accounts = o
	return s
}

// OpenCurrencyAccount implements onboardingv1connect.OnboardingServiceHandler.
func (s *Service) OpenCurrencyAccount(ctx context.Context, req *connect.Request[onboardingv1.OpenCurrencyAccountRequest]) (*connect.Response[onboardingv1.OpenCurrencyAccountResponse], error) {
	if s.accounts == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("account opening is not configured"))
	}
	acc, err := s.accounts.Open(ctx, req.Msg.PartyId, req.Msg.Asset, req.Msg.Actor)
	switch {
	case errors.Is(err, accounts.ErrInvalid):
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, accounts.ErrNotActive), errors.Is(err, licence.ErrNotApproved):
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	case err != nil:
		if s.log != nil {
			s.log.Error("open currency account failed", "asset", req.Msg.Asset, "err", err)
		}
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("the account could not be opened now; try again"))
	}
	return connect.NewResponse(&onboardingv1.OpenCurrencyAccountResponse{
		AccountId: acc.AccountID, Asset: acc.Asset, Reference: acc.Reference, Partner: acc.Partner,
	}), nil
}
