const http = require('http');
fetch('http://localhost:9011/api/v1/login', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({username: 'root', password: 'minioadmin'})
}).then(res => {
    const cookie = res.headers.get('set-cookie').split(';')[0];
    return fetch('http://localhost:9011/_admin/presign', {
        method: 'POST',
        headers: {'Content-Type': 'application/json', 'Cookie': cookie},
        body: JSON.stringify({method: 'PUT', bucket: 'mybucket', key: 'def2.jpg', expires: 3600})
    }).then(res => res.json()).then(data => {
        console.log(data.url);
        return fetch(data.url, {
            method: 'PUT',
            headers: {'x-amz-copy-source': '/mybucket/def.jpg'}
        });
    });
}).then(async res => {
    console.log(res.status, await res.text());
});
