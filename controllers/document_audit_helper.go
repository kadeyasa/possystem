package controllers

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kadeyasa/possystem/models"
	"github.com/kadeyasa/possystem/services"
	"gorm.io/gorm"
)

const (
	documentAuditActionCreated           = "created"
	documentAuditActionSubmitted         = "submitted"
	documentAuditActionApproved          = "approved"
	documentAuditActionRejected          = "rejected"
	documentAuditActionVoided            = "voided"
	documentAuditActionCreatedFromSource = "created_from_source"
)

type documentAuditInput struct {
	OutletID     uint
	DocumentType string
	DocumentID   uint
	Action       string
	Summary      string
	Note         string
	Metadata     interface{}
	Before       interface{}
	After        interface{}
}

func marshalAuditPayload(value interface{}) string {
	if value == nil {
		return ""
	}

	bytes, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(bytes)
}

func recordDocumentAudit(tx *gorm.DB, c *gin.Context, input documentAuditInput) error {
	if tx == nil || c == nil {
		return nil
	}
	if err := services.EnsureDocumentAuditSchema(tx); err != nil {
		return err
	}

	actor := readApprovalActorSnapshot(c)
	entry := models.DocumentAuditTrail{
		OutletID:     input.OutletID,
		DocumentType: strings.TrimSpace(input.DocumentType),
		DocumentID:   input.DocumentID,
		Action:       strings.TrimSpace(input.Action),
		Summary:      strings.TrimSpace(input.Summary),
		Note:         strings.TrimSpace(input.Note),
		Metadata:     marshalAuditPayload(input.Metadata),
		BeforeState:  marshalAuditPayload(input.Before),
		AfterState:   marshalAuditPayload(input.After),
		ActorUserID:  strings.TrimSpace(actor.UserID),
		ActorType:    strings.TrimSpace(actor.ActorType),
		ActorName:    strings.TrimSpace(actor.Name),
		RequestPath:  c.FullPath(),
		IPAddress:    c.ClientIP(),
		UserAgent:    strings.TrimSpace(c.Request.UserAgent()),
	}

	return tx.Create(&entry).Error
}
