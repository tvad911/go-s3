import re

content = open("web/dist/app.js", "r", encoding="utf-8").read()

# 1. Add selectedObjects global
content = content.replace("let currentUser = null;", "let currentUser = null;\nlet selectedObjects = new Set();")

# 2. Clear selectedObjects in fetchObjects
content = content.replace("async function fetchObjects() {", "async function fetchObjects() {\n    selectedObjects.clear();\n    if(window.updateBulkActionsUI) updateBulkActionsUI();")

# 3. Add the multi-select functions
new_funcs = """
// ==================== Multi-Select & Bulk Actions ====================
window.toggleObjectSelection = (e, key) => {
    e.stopPropagation();
    if (e.target.checked) selectedObjects.add(key);
    else selectedObjects.delete(key);
    updateBulkActionsUI();
    
    // Update header checkbox
    const allCheckboxes = document.querySelectorAll('#objects-table-body input[type="checkbox"]');
    const allChecked = Array.from(allCheckboxes).every(cb => cb.checked) && allCheckboxes.length > 0;
    const selectAllCb = document.getElementById("select-all-objects");
    if (selectAllCb) selectAllCb.checked = allChecked;
};

window.toggleSelectAllObjects = () => {
    const selectAllCb = document.getElementById("select-all-objects");
    const isChecked = selectAllCb ? selectAllCb.checked : false;
    const checkboxes = document.querySelectorAll('#objects-table-body input[type="checkbox"]');
    
    checkboxes.forEach(cb => {
        cb.checked = isChecked;
        const match = cb.getAttribute("onclick").match(/'([^']+)'/);
        if (match) {
            if (isChecked) selectedObjects.add(match[1]);
            else selectedObjects.delete(match[1]);
        }
    });
    updateBulkActionsUI();
};

window.updateBulkActionsUI = () => {
    const bulkActions = document.getElementById("bulk-actions");
    const selectedCountSpan = document.getElementById("selected-count");
    if (bulkActions && selectedCountSpan) {
        if (selectedObjects.size > 0) {
            bulkActions.style.display = "flex";
            selectedCountSpan.textContent = selectedObjects.size;
        } else {
            bulkActions.style.display = "none";
        }
    }
};

window.bulkDeleteObjects = async () => {
    if (selectedObjects.size === 0) return;
    if (!confirm(`Delete ${selectedObjects.size} selected items?`)) return;
    
    try {
        const keys = Array.from(selectedObjects);
        await api("POST", `/_admin/buckets/${currentBucket}/objects/delete`, keys);
        showToast(`Deleted ${keys.length} items`, "success");
        fetchObjects();
    } catch (err) {
        showToast(err.message, "error");
    }
};

window.bulkDownloadObjects = async () => {
    if (selectedObjects.size === 0) return;
    showToast(`Starting download for ${selectedObjects.size} items...`, "info");
    for (const key of selectedObjects) {
        try {
            const data = await api("POST", "/_admin/presign", { method: "GET", bucket: currentBucket, key, expires: 3600 });
            const a = document.createElement("a");
            a.href = data.url;
            a.download = key.split("/").pop() || "download";
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            await new Promise(r => setTimeout(r, 200));
        } catch (err) {
            showToast(`Failed to download ${key}: ${err.message}`, "error");
        }
    }
};
"""

content = content.replace("// ==================== Bucket Settings ====================", new_funcs + "\n// ==================== Bucket Settings ====================")

# 4. Fix folder-input logic
upload_js = """
// Upload
const uploadZone = document.getElementById('upload-zone');
const fileInput = document.getElementById('file-input');
const folderInput = document.getElementById('folder-input');
if (uploadZone) {
    uploadZone.addEventListener('click', (e) => {
        if (e.target.tagName !== 'BUTTON') fileInput.click();
    });
    uploadZone.addEventListener('dragover', (e) => { e.preventDefault(); uploadZone.classList.add('dragover'); });
    uploadZone.addEventListener('dragleave', () => uploadZone.classList.remove('dragover'));
    uploadZone.addEventListener('drop', (e) => { 
        e.preventDefault(); 
        uploadZone.classList.remove('dragover'); 
        if (e.dataTransfer.files.length) handleUploads(e.dataTransfer.files); 
    });
}
if (fileInput) fileInput.addEventListener('change', () => { if (fileInput.files.length) handleUploads(fileInput.files); });
if (folderInput) folderInput.addEventListener('change', () => { if (folderInput.files.length) handleUploads(folderInput.files); });

async function handleUploads(files) {
    if (!currentBucket) { showToast('Select a bucket first', 'error'); return; }
    for (const file of files) {
        try {
            const relativePath = file.webkitRelativePath || file.name;
            showToast(`Uploading ${relativePath}...`, 'info');
            const key = `${currentPrefix}${relativePath}`;
            const data = await api('POST', '/_admin/presign', { method: 'PUT', bucket: currentBucket, key, expires: 3600 });
            const res = await fetch(data.url, { method: 'PUT', body: file });
            if (!res.ok) throw new Error(`Upload failed: ${res.status}`);
            showToast(`${relativePath} uploaded`, 'success');
        } catch (err) { showToast(err.message, 'error'); }
    }
    fetchObjects();
}
"""

content = re.sub(r"// Upload.*?fetchObjects\(\);\n}", upload_js, content, flags=re.DOTALL)

open("web/dist/app.js", "w", encoding="utf-8").write(content)
print("Updated app.js")
