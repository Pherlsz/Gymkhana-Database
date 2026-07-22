package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type fakeAttachmentService struct {
	input attachment.CreateUploadIntentInput
	grant attachment.UploadGrant
	err   error
}

func (service *fakeAttachmentService) CreateUploadIntent(_ context.Context, _ auth.Session, input attachment.CreateUploadIntentInput, _ string) (attachment.UploadGrant, error) {
	service.input = input
	return service.grant, service.err
}

func (service *fakeAttachmentService) ConfirmUpload(context.Context, auth.Session, attachment.Identifier, string) (attachment.Attachment, error) {
	return attachment.Attachment{}, service.err
}

func (service *fakeAttachmentService) List(context.Context, auth.Session, attachment.OwnerReference, bool) ([]attachment.Attachment, error) {
	return nil, service.err
}

func (service *fakeAttachmentService) Download(context.Context, auth.Session, attachment.Identifier, string) (attachment.SignedRequest, error) {
	return attachment.SignedRequest{}, service.err
}

func (service *fakeAttachmentService) Trash(context.Context, auth.Session, attachment.Identifier, int64, string, string) (attachment.Attachment, error) {
	return attachment.Attachment{}, service.err
}

func (service *fakeAttachmentService) Restore(context.Context, auth.Session, attachment.Identifier, int64, string) (attachment.Attachment, error) {
	return attachment.Attachment{}, service.err
}

func attachmentHTTPFixture(t *testing.T) (*fakeAdministrationService, *fakeAttachmentService, attachment.Identifier, attachment.Identifier, http.Handler) {
	t.Helper()
	actorID, _ := auth.NewIdentifier()
	ownerID, _ := attachment.NewIdentifier()
	intentID, _ := attachment.NewIdentifier()
	now := time.Now().UTC()
	authentication := &fakeAdministrationService{fakeAuthenticationService: fakeAuthenticationService{session: auth.Session{User: auth.User{
		ID: actorID, Email: "member@example.com", Role: auth.RoleMember, Active: true,
	}}}}
	service := &fakeAttachmentService{grant: attachment.UploadGrant{
		Intent: attachment.UploadIntent{ID: intentID, ExpiresAt: now.Add(10 * time.Minute)},
		Upload: attachment.SignedRequest{URL: "https://upload.invalid/private", Method: http.MethodPut, Headers: map[string]string{"Content-Type": "application/pdf"}, ExpiresAt: now.Add(10 * time.Minute)},
	}}
	return authentication, service, ownerID, intentID, New(authTestLogger(), nil, Options{Auth: authentication, Attachment: service})
}

func TestAttachmentUploadIntentRouteAndProtectionErrors(t *testing.T) {
	authentication, service, ownerID, intentID, handler := attachmentHTTPFixture(t)
	body := `{"owner_kind":"DOCUMENT","owner_id":"` + ownerID.String() + `","original_filename":"proof.pdf","declared_mime":"application/pdf","expected_size":123}`

	request := httptest.NewRequest(http.MethodPost, "/api/v1/attachment-upload-intents", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || service.input.Owner.ID != ownerID || service.input.ExpectedSize != 123 || !strings.Contains(response.Body.String(), intentID.String()) {
		t.Fatalf("create status = %d, input = %#v, body = %s", response.Code, service.input, response.Body.String())
	}

	for _, test := range []struct {
		name string
		err  error
		code string
	}{
		{name: "rate", err: attachment.ErrUploadRateLimited, code: "rate_limited"},
		{name: "quota", err: attachment.ErrStorageQuotaExceeded, code: "quota_exceeded"},
	} {
		t.Run(test.name, func(t *testing.T) {
			service.err = test.err
			request := httptest.NewRequest(http.MethodPost, "/api/v1/attachment-upload-intents", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), test.code) {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}

	service.err = attachment.ErrForbidden
	request = httptest.NewRequest(http.MethodPost, "/api/v1/attachment-upload-intents", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("forbidden status = %d, body = %s", response.Code, response.Body.String())
	}

	service.err = nil
	authentication.sessionErr = auth.ErrUnauthenticated
	request = httptest.NewRequest(http.MethodPost, "/api/v1/attachment-upload-intents", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, body = %s", response.Code, response.Body.String())
	}
}
