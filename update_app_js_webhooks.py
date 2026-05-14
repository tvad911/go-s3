import re

content = open("web/dist/app.js", "r", encoding="utf-8").read()

# 1. Update switchBucketSettingsTab
old_switch = """window.switchBucketSettingsTab = (tab) => {
    ['overview', 'access', 'cors', 'lifecycle', 'website', 'domains'].forEach(t => {
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
    } else if (tab === 'domains') {
        fetchBucketCustomDomains();
    }
};"""

new_switch = """window.switchBucketSettingsTab = (tab) => {
    ['overview', 'access', 'cors', 'lifecycle', 'website', 'domains', 'webhooks'].forEach(t => {
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
    } else if (tab === 'domains') {
        fetchBucketCustomDomains();
    } else if (tab === 'webhooks') {
        fetchBucketWebhooks();
    }
};"""

content = content.replace(old_switch, new_switch)

# 2. Add Webhook functions
webhook_funcs = """
async function fetchBucketWebhooks() {
    try {
        const config = await api('GET', `/_admin/buckets/${currentBucket}/notifications`);
        const textarea = document.getElementById('bucket-webhooks-json');
        if (!config || !config.webhooks || config.webhooks.length === 0) {
            textarea.value = '';
        } else {
            textarea.value = JSON.stringify(config, null, 2);
        }
    } catch (err) {
        showToast('Failed to fetch webhooks: ' + err.message, 'error');
    }
}

window.addBucketWebhookTemplate = () => {
    const template = {
        webhooks: [
            {
                events: ["s3:ObjectCreated:*", "s3:ObjectRemoved:*"],
                url: "https://your-api.com/webhook",
                secret: "your_optional_hmac_secret"
            }
        ]
    };
    document.getElementById('bucket-webhooks-json').value = JSON.stringify(template, null, 2);
    document.getElementById('bucket-webhooks-error').style.display = 'none';
};

window.saveBucketWebhooks = async () => {
    const textarea = document.getElementById('bucket-webhooks-json');
    const val = textarea.value.trim();
    const errorEl = document.getElementById('bucket-webhooks-error');
    
    if (!val) {
        try {
            await api('DELETE', `/_admin/buckets/${currentBucket}/notifications`);
            showToast('Webhooks deleted successfully', 'success');
        } catch (err) {
            showToast(err.message, 'error');
        }
        return;
    }

    try {
        const payload = JSON.parse(val);
        await api('PUT', `/_admin/buckets/${currentBucket}/notifications`, payload);
        showToast('Webhooks saved successfully', 'success');
    } catch (err) {
        errorEl.textContent = 'Invalid JSON: ' + err.message;
        errorEl.style.display = 'block';
    }
};

// ==================== Connection Info ====================
"""

content = content.replace("// ==================== Connection Info ====================", webhook_funcs)

open("web/dist/app.js", "w", encoding="utf-8").write(content)
print("Updated app.js for webhooks")
