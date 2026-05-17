# PROJECT PLAN: File Manager v2 Embedded & Bucket Settings Active States

## Phase 1: Phân tích & Yêu cầu (Analysis)
- **Mục tiêu 1:** Nâng cấp UI của "File Manager v2" (Upload Staging). Thay vì hiển thị bảng hàng đợi (Queue) dưới dạng cửa sổ Modal (Popup) tách biệt, người dùng muốn nhúng trực tiếp (embed) bảng danh sách chờ upload này vào ngay khu vực `upload-zone` (khung chọn file/folder phía trên danh sách object). Điều này giúp trải nghiệm mượt mà hơn, không bị che khuất tầm nhìn bởi modal.
- **Mục tiêu 2:** Sửa lỗi UX trong phần Bucket Settings. Khi người dùng chuyển đổi qua lại giữa các menu tab (ví dụ: Overview, Policies, CORS...), tab đang được chọn phải đổi trạng thái (active) và đổi màu/hiệu ứng để người dùng biết mình đang ở đâu.

## Phase 2: Giải pháp kiến trúc (Planning)

### 1. Embedded Upload Staging (File Manager)
- **HTML:** 
  - Di chuyển mã HTML của bảng `staging-table` và thanh `staging-progress-bar` từ `#upload-staging-modal` sang thẳng bên trong `#upload-zone` (hoặc tạo một block mới `staging-area` nằm ngay dưới/thay thế `upload-zone` khi có file được chọn).
  - Khung `upload-zone` mặc định vẫn giữ nguyên (Drag & Drop, nút +File, +Folder). Khi người dùng chọn file, bảng danh sách chờ sẽ tự động mở rộng ra (expand) ở ngay khu vực đó thay vì bật Modal.
- **JS (`app.js`):**
  - Xóa bỏ việc gọi `openModal('upload-staging-modal')` khi `stageFiles()` kích hoạt.
  - Cập nhật hàm `renderStagingQueue()` để thao tác trực tiếp với DOM trong khu vực `upload-zone`.
  - Thay đổi nút "Start Upload" và "Clear" để nằm gọn trong giao diện embedded này.

### 2. Tab Active States (Bucket Settings)
- **JS/CSS:**
  - Trong hàm chuyển đổi màn hình Bucket Settings (thường là `openBucketSettings` hoặc hàm chuyển tab nội bộ `switchSettingsTab` nếu có).
  - Tìm tất cả các phần tử đại diện cho menu tab (thường có class `.tab-link` hoặc thẻ `<li>` menu nội bộ).
  - Gỡ class `active` (hoặc `sort-active`) khỏi tất cả các tab khác và thêm class `active` vào tab vừa được click.
  - Đảm bảo trong `style.css`, class `active` của các settings tabs được định nghĩa rõ ràng (ví dụ: đổi màu viền dưới (border-bottom), chữ sáng hơn, text color `var(--accent-primary)`).

## Phase 3: Check-list & Tiêu chí nghiệm thu (Verification)
- [ ] Kéo thả 1 file vào màn hình Objects: Bảng hàng đợi file hiện ra ngay trong trang (không có Popup Modal che lấp).
- [ ] Bấm nút "Clear" hàng đợi: Bảng hàng đợi thu gọn lại, trả lại khung Drag & Drop mặc định.
- [ ] Vào Settings của 1 Bucket: Click qua lại giữa "Policies" và "CORS" -> tab hiển thị đúng trạng thái Active (màu highlight).
- [ ] Test lại giao diện trên màn hình Mobile để đảm bảo bảng Staging nhúng không bị vỡ bố cục.

## Phân công (Agent Assignments)
- **Frontend Specialist:** Thực hiện thay đổi HTML trong `index.html`, CSS trong `style.css`, và JS event listeners trong `app.js`.

---
> ⚠️ Kế hoạch này được tạo tự động bởi `project-planner`. KHÔNG có dòng code logic nào bị thay đổi.
