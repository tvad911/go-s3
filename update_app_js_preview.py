import re

content = open("web/dist/app.js", "r", encoding="utf-8").read()

old_func = """window.previewObject = async (key) => {
    try {
        const title = document.getElementById('preview-title');
        const img = document.getElementById('preview-image');
        const loader = document.getElementById('preview-loading');
        
        title.textContent = key;
        img.style.display = 'none';
        loader.style.display = 'block';
        openModal('preview-modal');

        // Get short-lived URL for preview
        const data = await api('POST', '/_admin/presign', { method: 'GET', bucket: currentBucket, key, expires: 3600 });
        
        img.onload = () => {
            loader.style.display = 'none';
            img.style.display = 'block';
        };
        img.onerror = () => {
            loader.style.display = 'none';
            title.textContent = 'Preview failed';
        };
        img.src = data.url;
    } catch (err) { 
        closeModal('preview-modal');
        showToast(err.message, 'error'); 
    }
};"""

new_func = """window.previewObject = async (key) => {
    try {
        const title = document.getElementById('preview-title');
        const img = document.getElementById('preview-image');
        const video = document.getElementById('preview-video');
        const audio = document.getElementById('preview-audio');
        const text = document.getElementById('preview-text');
        const loader = document.getElementById('preview-loading');
        
        title.textContent = key;
        img.style.display = 'none';
        if (video) video.style.display = 'none';
        if (audio) audio.style.display = 'none';
        if (text) text.style.display = 'none';
        
        // Pause any playing media
        if (video) { video.pause(); video.removeAttribute('src'); video.load(); }
        if (audio) { audio.pause(); audio.removeAttribute('src'); audio.load(); }
        
        loader.style.display = 'block';
        openModal('preview-modal');

        // Get short-lived URL for preview
        const data = await api('POST', '/_admin/presign', { method: 'GET', bucket: currentBucket, key, expires: 3600 });
        
        const ext = key.split('.').pop().toLowerCase();
        const imageExts = ['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp', 'ico'];
        const videoExts = ['mp4', 'webm', 'ogg', 'mov'];
        const audioExts = ['mp3', 'wav', 'ogg', 'aac', 'flac'];
        const textExts = ['txt', 'md', 'csv', 'json', 'xml', 'js', 'html', 'css', 'go', 'py', 'sh', 'yaml', 'yml'];
        
        if (imageExts.includes(ext)) {
            img.onload = () => {
                loader.style.display = 'none';
                img.style.display = 'block';
            };
            img.onerror = () => {
                loader.style.display = 'none';
                title.textContent = 'Preview failed';
            };
            img.src = data.url;
        } else if (videoExts.includes(ext)) {
            loader.style.display = 'none';
            video.style.display = 'block';
            video.src = data.url;
            video.play().catch(e => console.error("Auto-play prevented", e));
        } else if (audioExts.includes(ext)) {
            loader.style.display = 'none';
            audio.style.display = 'block';
            audio.src = data.url;
            audio.play().catch(e => console.error("Auto-play prevented", e));
        } else if (textExts.includes(ext)) {
            try {
                const response = await fetch(data.url);
                const textContent = await response.text();
                loader.style.display = 'none';
                text.style.display = 'block';
                text.textContent = textContent;
                text.style.textAlign = 'left';
                text.style.color = 'var(--text-primary)';
            } catch (e) {
                loader.style.display = 'none';
                title.textContent = 'Failed to load text content';
            }
        } else {
            // Fallback for unsupported types
            loader.style.display = 'none';
            text.style.display = 'block';
            text.textContent = 'Preview not supported for this file type.\\nPlease download the file to view its contents.';
            text.style.textAlign = 'center';
            text.style.color = 'var(--text-secondary)';
        }
    } catch (err) { 
        closeModal('preview-modal');
        showToast(err.message, 'error'); 
    }
};

// Also pause media when modal closes
const originalCloseModal = window.closeModal;
window.closeModal = (id) => {
    if (id === 'preview-modal') {
        const video = document.getElementById('preview-video');
        const audio = document.getElementById('preview-audio');
        if (video) { video.pause(); video.removeAttribute('src'); video.load(); }
        if (audio) { audio.pause(); audio.removeAttribute('src'); audio.load(); }
    }
    if (originalCloseModal) originalCloseModal(id);
};"""

content = content.replace(old_func, new_func)

open("web/dist/app.js", "w", encoding="utf-8").write(content)
print("Updated app.js for preview")
