package email

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"strings"

	"github.com/yusqohid/go-htmx-ecommerce/internal/domain"
)

var (
	// ErrMissingRecipient is returned when attempting to send an email without a valid recipient.
	ErrMissingRecipient = errors.New("recipient email address is required")
)

// Service coordinates generating and sending transactional emails for Sellora.
type Service struct {
	sender  Sender
	baseURL string
}

// NewService constructs a new email Service.
func NewService(sender Sender, baseURL string) *Service {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return &Service{
		sender:  sender,
		baseURL: baseURL,
	}
}

type receiptEmailData struct {
	Order       *domain.Order
	Customer    *domain.User
	TotalMoney  string
	DownloadURL string
	OrdersURL   string
}

const htmlReceiptTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Payment Receipt - {{ .Order.Reference }}</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background-color: #f8fafc; color: #1e293b; margin: 0; padding: 0; }
    .container { max-width: 600px; margin: 30px auto; background-color: #ffffff; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 6px rgba(0, 0, 0, 0.05); }
    .header { background-color: #0f172a; padding: 28px; text-align: center; color: #ffffff; }
    .header h1 { margin: 0; font-size: 24px; font-weight: 700; letter-spacing: -0.5px; }
    .header p { margin: 6px 0 0 0; color: #94a3b8; font-size: 14px; }
    .content { padding: 32px; }
    .badge { display: inline-block; background-color: #dcfce7; color: #15803d; font-size: 12px; font-weight: 600; padding: 4px 12px; border-radius: 9999px; text-transform: uppercase; margin-bottom: 16px; }
    .meta-box { background-color: #f8fafc; border: 1px solid #e2e8f0; border-radius: 8px; padding: 16px; margin-bottom: 24px; }
    .meta-row { display: flex; justify-content: space-between; margin-bottom: 8px; font-size: 14px; }
    .meta-row:last-child { margin-bottom: 0; }
    .meta-label { color: #64748b; }
    .meta-value { font-weight: 600; color: #0f172a; }
    .items-table { width: 100%; border-collapse: collapse; margin-bottom: 24px; }
    .items-table th { text-align: left; padding: 10px 0; border-bottom: 2px solid #e2e8f0; font-size: 13px; color: #64748b; text-transform: uppercase; }
    .items-table td { padding: 14px 0; border-bottom: 1px solid #f1f5f9; font-size: 15px; }
    .total-row td { border-bottom: none; font-size: 18px; font-weight: 700; color: #0f172a; padding-top: 16px; }
    .btn-container { text-align: center; margin: 32px 0; }
    .btn { display: inline-block; background-color: #10b981; color: #ffffff !important; text-decoration: none; padding: 14px 32px; border-radius: 9999px; font-size: 16px; font-weight: 600; box-shadow: 0 4px 12px rgba(16, 185, 129, 0.3); }
    .footer { background-color: #f8fafc; border-top: 1px solid #e2e8f0; padding: 24px; text-align: center; font-size: 13px; color: #94a3b8; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>Sellora</h1>
      <p>Digital Product Commerce</p>
    </div>
    <div class="content">
      <span class="badge">Payment Successful</span>
      <h2 style="margin: 0 0 8px 0; font-size: 20px; color: #0f172a;">Thank you for your purchase!</h2>
      <p style="margin: 0 0 24px 0; font-size: 15px; color: #475569; line-height: 1.5;">
        Hi {{ .Customer.Name }}, your payment has been confirmed. You now have lifetime access to download the digital assets included in your order.
      </p>

      <div class="meta-box">
        <table style="width: 100%;">
          <tr>
            <td style="color: #64748b; font-size: 14px; padding: 4px 0;">Order Reference:</td>
            <td style="font-weight: 600; font-family: monospace; text-align: right; color: #0f172a;">{{ .Order.Reference }}</td>
          </tr>
          <tr>
            <td style="color: #64748b; font-size: 14px; padding: 4px 0;">Payment Provider:</td>
            <td style="font-weight: 600; text-align: right; text-transform: uppercase; color: #0f172a;">{{ .Order.PaymentProvider }}</td>
          </tr>
          <tr>
            <td style="color: #64748b; font-size: 14px; padding: 4px 0;">Date:</td>
            <td style="font-weight: 600; text-align: right; color: #0f172a;">{{ .Order.CreatedAt.Format "02 Jan 2006, 15:04 MST" }}</td>
          </tr>
        </table>
      </div>

      <table class="items-table">
        <thead>
          <tr>
            <th>Product Item</th>
            <th style="text-align: right;">Price</th>
          </tr>
        </thead>
        <tbody>
          {{ range .Order.Items }}
          <tr>
            <td style="font-weight: 500;">{{ .ProductName }}</td>
            <td style="text-align: right; font-weight: 600; color: #0f172a;">{{ formatMoney .Price }}</td>
          </tr>
          {{ end }}
          <tr class="total-row">
            <td>Total Paid</td>
            <td style="text-align: right; color: #10b981;">{{ .TotalMoney }}</td>
          </tr>
        </tbody>
      </table>

      <div class="btn-container">
        <a href="{{ .DownloadURL }}" class="btn">Access &amp; Download Files</a>
      </div>

      <p style="font-size: 13px; color: #64748b; text-align: center; margin: 0;">
        You can also access your purchases at any time from your <a href="{{ .OrdersURL }}" style="color: #10b981; text-decoration: none;">Customer Account Dashboard</a>.
      </p>
    </div>
    <div class="footer">
      &copy; 2026 Sellora Digital Commerce Platform. All rights reserved.<br>
      This is an automated transaction receipt. Please do not reply directly to this email.
    </div>
  </div>
</body>
</html>`

// SendOrderReceipt compiles and transmits an order confirmation and digital asset receipt email.
func (s *Service) SendOrderReceipt(ctx context.Context, order *domain.Order) error {
	if order == nil {
		return errors.New("cannot send receipt for nil order")
	}
	if order.Customer == nil || strings.TrimSpace(order.Customer.Email) == "" {
		return ErrMissingRecipient
	}

	downloadURL := fmt.Sprintf("%s/orders/%s/success", s.baseURL, order.Reference)
	ordersURL := fmt.Sprintf("%s/account/orders", s.baseURL)
	formattedTotal := formatRupiah(order.TotalAmount)

	data := receiptEmailData{
		Order:       order,
		Customer:    order.Customer,
		TotalMoney:  formattedTotal,
		DownloadURL: downloadURL,
		OrdersURL:   ordersURL,
	}

	tmpl, err := template.New("order_receipt").Funcs(template.FuncMap{
		"formatMoney": formatRupiah,
	}).Parse(htmlReceiptTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse receipt email template: %w", err)
	}

	var htmlBuf bytes.Buffer
	if err := tmpl.Execute(&htmlBuf, data); err != nil {
		return fmt.Errorf("failed to execute receipt email template: %w", err)
	}

	var textBuf strings.Builder
	textBuf.WriteString(fmt.Sprintf("SELLORA - PAYMENT RECEIPT\n\n"))
	textBuf.WriteString(fmt.Sprintf("Hi %s,\n\n", order.Customer.Name))
	textBuf.WriteString("Thank you for your purchase! Your payment has been confirmed.\n\n")
	textBuf.WriteString(fmt.Sprintf("Order Reference: %s\n", order.Reference))
	textBuf.WriteString(fmt.Sprintf("Total Paid:      %s\n", formattedTotal))
	textBuf.WriteString(fmt.Sprintf("Payment Gateway: %s\n\n", strings.ToUpper(order.PaymentProvider)))
	textBuf.WriteString("Purchased Items:\n")
	for _, item := range order.Items {
		textBuf.WriteString(fmt.Sprintf("- %s: %s\n", item.ProductName, formatRupiah(item.Price)))
	}
	textBuf.WriteString(fmt.Sprintf("\nAccess & Download Files:\n%s\n\n", downloadURL))
	textBuf.WriteString(fmt.Sprintf("Customer Account:\n%s\n", ordersURL))

	msg := Message{
		To:       order.Customer.Email,
		Subject:  fmt.Sprintf("Receipt for Order %s - Sellora", order.Reference),
		HTMLBody: htmlBuf.String(),
		TextBody: textBuf.String(),
	}

	return s.sender.Send(ctx, msg)
}

func formatRupiah(amount int64) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}

	s := fmt.Sprintf("%d", amount)
	n := len(s)
	if n <= 3 {
		return fmt.Sprintf("%sRp %s", sign, s)
	}

	var parts []string
	remainder := n % 3
	if remainder > 0 {
		parts = append(parts, s[:remainder])
	}
	for i := remainder; i < n; i += 3 {
		parts = append(parts, s[i:i+3])
	}

	return fmt.Sprintf("%sRp %s", sign, strings.Join(parts, "."))
}
