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
- [x] **Customer Dashboard & Navigation:**
  - [x] Polish customer order history and digital download shelf UI.
- [x] **Core Features:**
  - [x] Email notifications upon successful order payment (receipt & download links) via `internal/email`.
  - [x] LYNK and Midtrans webhook integration.

---

## 🎯 Next Priorities (High Impact)

- [ ] **Product Media Gallery & Carousel:**
  - Support multiple product preview images (requires DB update for `product_images`).
  - Interactive carousel/lightbox on the product detail page.
  - Add video/demo embed player for digital goods (e.g., YouTube/Vimeo preview links).
- [ ] **Discount & Promotion Engine:**
  - Support automated discount codes and coupons (fixed amount or percentage).
  - Add coupon input field in the checkout flow.

---

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
