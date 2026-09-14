# TODO: Future Improvements & Technical Debt

This document tracks planned architectural, UI/UX, and functional enhancements to be addressed in future iterations.

---

## 🎨 UI / UX Enhancements (Future Sprints)

- [ ] **Storefront Visual Polish & Design System:**
  - Enhance public storefront typography, spacing, and micro-interactions.
  - Implement full design system tokens or dedicated frontend theme beyond basic Bootstrap layout.
  - Add skeleton loaders for HTMX catalog search and filtering (`hx-indicator`).
  - Add dark mode / theme toggle support.

- [ ] **Product Detail Page Improvements:**
  - Support multiple product preview images and interactive carousel/lightbox.
  - Add video/demo embed player for digital goods (e.g., video tutorials, preview clips).
  - Rich Markdown rendering for product descriptions (converting markdown to sanitized HTML).
  - Customer review and rating display component.

- [ ] **Customer Dashboard & Navigation:**
  - Polish customer order history and digital download shelf UI.
  - Add download progress indicators and expiration notices (if applicable).
  - Enhance mobile drawer navigation and touch responsiveness.

---

## 🚀 Future Feature Enhancements

- [ ] Support automated discount codes and coupons.
- [ ] Email notifications upon successful order payment (receipt & download links).
- [ ] Multi-currency display / automated currency conversion.
- [ ] S3/Cloudflare R2 storage adapter for digital product assets when server storage limit is approached.
