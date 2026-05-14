import re

content = open("web/dist/app.js", "r", encoding="utf-8").read()

# 1. Update switchBucketSettingsTab
old_switch = """window.switchBucketSettingsTab = (tab) => {
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

new_switch = """window.switchBucketSettingsTab = (tab) => {
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

content = content.replace(old_switch, new_switch)

# 2. Add Domains functions
domain_funcs = """
async function fetchBucketCustomDomains() {
    try {
        const domains = await api('GET', `/_admin/buckets/${currentBucket}/domains`);
        renderBucketCustomDomains(domains || []);
    } catch (err) {
        showToast('Failed to fetch custom domains: ' + err.message, 'error');
    }
}

function renderBucketCustomDomains(domains) {
    const list = document.getElementById('bucket-domains-list');
    if (domains.length === 0) {
        list.innerHTML = `<div style="color:var(--text-secondary);font-style:italic;padding:0.5rem 0;">No custom domains mapped to this bucket.</div>`;
        return;
    }
    
    list.innerHTML = domains.map(d => `
        <div style="display:flex;justify-content:space-between;align-items:center;background:var(--bg-tertiary);padding:0.75rem 1rem;border-radius:4px;border:1px solid rgba(255,255,255,0.05);">
            <div style="display:flex;align-items:center;gap:0.5rem;">
                <svg viewBox="0 0 24 24" fill="none" stroke="var(--accent-primary)" stroke-width="2" style="width:16px;height:16px;"><circle cx="12" cy="12" r="10"></circle><line x1="2" y1="12" x2="22" y2="12"></line><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"></path></svg>
                <a href="http://${d}" target="_blank" style="color:var(--text-primary);text-decoration:none;">${d}</a>
            </div>
            <button class="btn btn-ghost text-danger btn-sm" onclick="deleteBucketCustomDomain('${d}')" style="padding:0.25rem 0.5rem;">Remove</button>
        </div>
    `).join('');
}

window.addBucketCustomDomain = async () => {
    const input = document.getElementById('new-custom-domain');
    const domain = input.value.trim();
    if (!domain) return;
    
    try {
        await api('PUT', `/_admin/buckets/${currentBucket}/domains`, { domain });
        input.value = '';
        showToast('Domain mapped successfully', 'success');
        fetchBucketCustomDomains();
    } catch (err) {
        showToast(err.message, 'error');
    }
};

window.deleteBucketCustomDomain = async (domain) => {
    if (!confirm(`Are you sure you want to remove the domain mapping for ${domain}?`)) return;
    
    try {
        await api('DELETE', `/_admin/buckets/${currentBucket}/domains/${encodeURIComponent(domain)}`);
        showToast('Domain mapping removed', 'success');
        fetchBucketCustomDomains();
    } catch (err) {
        showToast(err.message, 'error');
    }
};

// ==================== Connection Info ====================
"""

content = content.replace("// ==================== Connection Info ====================", domain_funcs)

open("web/dist/app.js", "w", encoding="utf-8").write(content)
print("Updated app.js for domains")
