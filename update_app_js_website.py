import re

content = open("web/dist/app.js", "r", encoding="utf-8").read()

# 1. Update switchBucketSettingsTab
old_switch = """window.switchBucketSettingsTab = (tab) => {
    ['overview', 'access', 'cors', 'lifecycle'].forEach(t => {
        const link = document.getElementById(`tab-link-${t}`);
        const content = document.getElementById(`tab-content-${t}`);
        if(link) link.classList.remove('active');
        if(content) content.style.display = 'none';
    });
    
    const activeLink = document.getElementById(`tab-link-${tab}`);
    const activeContent = document.getElementById(`tab-content-${tab}`);
    if(activeLink) activeLink.classList.add('active');
    if(activeContent) activeContent.style.display = 'block';
    
    if (tab === 'overview') {
        fetchBucketStats();
    } else if (tab === 'access') {
        fetchBucketPolicy();
    } else if (tab === 'cors') {
        fetchBucketCORS();
    } else if (tab === 'lifecycle') {
        fetchBucketLifecycle();
    }
};"""

new_switch = """window.switchBucketSettingsTab = (tab) => {
    ['overview', 'access', 'cors', 'lifecycle', 'website'].forEach(t => {
        const link = document.getElementById(`tab-link-${t}`);
        const content = document.getElementById(`tab-content-${t}`);
        if(link) link.classList.remove('active');
        if(content) content.style.display = 'none';
    });
    
    const activeLink = document.getElementById(`tab-link-${tab}`);
    const activeContent = document.getElementById(`tab-content-${tab}`);
    if(activeLink) activeLink.classList.add('active');
    if(activeContent) activeContent.style.display = 'block';
    
    if (tab === 'overview') {
        fetchBucketStats();
    } else if (tab === 'access') {
        fetchBucketPolicy();
    } else if (tab === 'cors') {
        fetchBucketCORS();
    } else if (tab === 'lifecycle') {
        fetchBucketLifecycle();
    } else if (tab === 'website') {
        fetchBucketWebsite();
    }
};"""

content = content.replace(old_switch, new_switch)

# 2. Add Website functions
website_funcs = """
async function fetchBucketWebsite() {
    try {
        const res = await api('GET', `/_admin/buckets/${currentBucket}/website`);
        document.getElementById('bucket-website-json').value = JSON.stringify(res, null, 2);
    } catch (err) {
        if (err.message.includes('not found')) {
            document.getElementById('bucket-website-json').value = '';
        } else {
            showToast('Failed to fetch website config: ' + err.message, 'error');
        }
    }
}

window.setBucketWebsitePreset = (type) => {
    const website = {
        "IndexDocument": {
            "Suffix": "index.html"
        },
        "ErrorDocument": {
            "Key": "error.html"
        }
    };
    const textarea = document.getElementById('bucket-website-json');
    if (type === 'default') {
        textarea.value = JSON.stringify(website, null, 2);
    } else {
        textarea.value = '';
    }
    validateJSONConfig(textarea, 'bucket-website-error');
};

window.saveBucketWebsite = async () => {
    const textarea = document.getElementById('bucket-website-json');
    const wsStr = textarea.value.trim();
    
    if (wsStr && !validateJSONConfig(textarea, 'bucket-website-error')) {
        showToast('Please fix JSON errors before saving', 'error');
        return;
    }

    try {
        if (!wsStr) {
            await api('DELETE', `/_admin/buckets/${currentBucket}/website`);
            showToast('Website config cleared', 'success');
        } else {
            const wsObj = JSON.parse(wsStr);
            await api('PUT', `/_admin/buckets/${currentBucket}/website`, wsObj);
            showToast('Website config saved', 'success');
        }
    } catch (err) {
        showToast(err.message, 'error');
    }
};

// ==================== Connection Info ====================
"""

content = content.replace("// ==================== Connection Info ====================", website_funcs)

open("web/dist/app.js", "w", encoding="utf-8").write(content)
print("Updated app.js for website")
