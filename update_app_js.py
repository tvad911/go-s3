import re

content = open("web/dist/app.js", "r", encoding="utf-8").read()

# 1. Update switchBucketSettingsTab
old_switch = """window.switchBucketSettingsTab = (tab) => {
    document.getElementById('tab-link-overview').classList.remove('active');
    document.getElementById('tab-link-access').classList.remove('active');
    document.getElementById('tab-link-cors').classList.remove('active');
    document.getElementById('tab-content-overview').style.display = 'none';
    document.getElementById('tab-content-access').style.display = 'none';
    document.getElementById('tab-content-cors').style.display = 'none';
    
    document.getElementById(`tab-link-${tab}`).classList.add('active');
    document.getElementById(`tab-content-${tab}`).style.display = 'block';
    
    if (tab === 'overview') {
        fetchBucketStats();
    } else if (tab === 'access') {
        fetchBucketPolicy();
    } else if (tab === 'cors') {
        fetchBucketCORS();
    }
};"""

new_switch = """window.switchBucketSettingsTab = (tab) => {
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

content = content.replace(old_switch, new_switch)

# 2. Add Lifecycle functions
lifecycle_funcs = """
async function fetchBucketLifecycle() {
    try {
        const res = await api('GET', `/_admin/buckets/${currentBucket}/lifecycle`);
        document.getElementById('bucket-lifecycle-json').value = JSON.stringify(res, null, 2);
    } catch (err) {
        if (err.message.includes('not found')) {
            document.getElementById('bucket-lifecycle-json').value = '';
        } else {
            showToast('Failed to fetch lifecycle: ' + err.message, 'error');
        }
    }
}

window.setBucketLifecyclePreset = (type) => {
    const lifecycle = {
        "Rules": [
            {
                "ID": "ExpireAfter30Days",
                "Status": "Enabled",
                "Filter": {
                    "Prefix": ""
                },
                "Expiration": {
                    "Days": 30
                }
            }
        ]
    };
    const textarea = document.getElementById('bucket-lifecycle-json');
    if (type === 'expire-30d') {
        textarea.value = JSON.stringify(lifecycle, null, 2);
    } else {
        textarea.value = '';
    }
    validateJSONConfig(textarea, 'bucket-lifecycle-error');
};

window.saveBucketLifecycle = async () => {
    const textarea = document.getElementById('bucket-lifecycle-json');
    const lcStr = textarea.value.trim();
    
    if (lcStr && !validateJSONConfig(textarea, 'bucket-lifecycle-error')) {
        showToast('Please fix JSON errors before saving', 'error');
        return;
    }

    try {
        if (!lcStr) {
            await api('DELETE', `/_admin/buckets/${currentBucket}/lifecycle`);
            showToast('Lifecycle cleared', 'success');
        } else {
            const lcObj = JSON.parse(lcStr);
            await api('PUT', `/_admin/buckets/${currentBucket}/lifecycle`, lcObj);
            showToast('Lifecycle saved', 'success');
        }
    } catch (err) {
        showToast(err.message, 'error');
    }
};

// ==================== Connection Info ====================
"""

content = content.replace("// ==================== Connection Info ====================", lifecycle_funcs)

open("web/dist/app.js", "w", encoding="utf-8").write(content)
print("Updated app.js for lifecycle")
