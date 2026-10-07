# TODO: Future Improvements & Technical Debt

This document tracks planned architectural, UI/UX, and functional enhancements. It has been updated to reflect the completion of the MVP and the Spark Admin Pro UI overhaul.

---

## ✅ Completed (MVP & UI Overhaul)

- [x] **Storefront Visual Polish & Design System:**
  - [x] Enhance public storefront typography, spacing, and micro-interactions (Aligned with Spark Admin Pro).
  - [x] Implement full design system tokens (Tokens CSS integrated).
  - [x] Add loading indicators for HTMX catalog search (`hx-indicator`).
- [x] **Product Detail Page Improvements:**
  - [x] Rich Markdown rendering for product descriptions (`RenderMarkdown` implemented).
- [x] **Product Media Gallery & Video Demo:**
  - [x] Multi-image gallery support with `product_images` schema migration and repository.
  - [x] Interactive thumbnail switcher on the product detail page.
  - [x] Video preview player (YouTube & Vimeo embed) for digital goods (`demo_url`).
- [x] **Customer Dashboard & Navigation:**
  - [x] Polish customer order history and digital download shelf UI.
- [x] **Core Features:**
  - [x] Email notifications upon successful order payment (receipt & download links) via `internal/email`.
  - [x] LYNK and Midtrans webhook integration.

---

- [x] **Discount & Promotion Engine:**
  - [x] Automated discount codes and coupons (fixed amount & percentage with cap, usage limit, and validity range).
  - [x] Admin dashboard coupon management (create, edit, toggle active status, delete).
  - [x] Reactive storefront checkout coupon application via HTMX (`apply-coupon`, `remove-coupon`, and query params).
  - [x] Webhook idempotency and state transition guards against duplicate side-effects.

---

## 🎯 Next Priorities (High Impact)

## 🚀 Future Feature Enhancements (Backlog)

- [ ] **Customer Trust & Engagement:**
  - Customer review and rating display component (only for verified buyers).
- [ ] **Infrastructural Scaling:**
  - S3 / Cloudflare R2 storage adapter for digital product assets when server storage limit is approached.
- [ ] **UI/UX & Customer Experience:**
  - Add dark mode / theme toggle support.
  - Add download progress indicators and expiration notices for downloaded files.
  - Enhance mobile drawer navigation and touch responsiveness.
- [ ] **Business Logic:**
  - Multi-currency display / automated currency conversion.
