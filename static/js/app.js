document.addEventListener('DOMContentLoaded', () => {
    const searchForm = document.getElementById('searchForm');
    const searchInput = document.getElementById('searchInput');
    const fileInput = document.getElementById('fileInput');
    const resultsDiv = document.getElementById('results');
    const docCountDiv = document.getElementById('docCount');
    const loadingDiv = document.getElementById('loading');
    const progressDiv = document.getElementById('uploadProgress');
    const progressFill = progressDiv.querySelector('.progress-fill');
    const progressText = progressDiv.querySelector('.progress-text');

    const showLoading = () => loadingDiv.style.display = 'block';
    const hideLoading = () => loadingDiv.style.display = 'none';

    const showMessage = (text, isError = false) => {
        const message = document.createElement('div');
        message.className = `message ${isError ? 'error' : 'success'}`;
        message.textContent = text;
        document.body.appendChild(message);
        setTimeout(() => message.remove(), 3000);
    };

    const updateDocCount = async () => {
        try {
            const response = await fetch('/api/status');
            const data = await response.json();
            docCountDiv.textContent = data.documents || '0';
        } catch (error) {
            console.error('Failed to update document count:', error);
        }
    };

    const handleFileUpload = async (files) => {
        console.log('Starting file upload for', files.length, 'files');
        showLoading();
        progressDiv.style.display = 'block';
        
        let successCount = 0;
        let failCount = 0;

        for (let i = 0; i < files.length; i++) {
            const file = files[i];
            progressText.textContent = `Uploading ${i + 1}/${files.length}: ${file.name}`;
            progressFill.style.width = `${(i / files.length) * 100}%`;

            const formData = new FormData();
            formData.append('file', file);

            try {
                console.log('Sending upload request for:', file.name);
                const response = await fetch('/api/upload', {
                    method: 'POST',
                    body: formData
                });

                console.log('Upload response status:', response.status);
                const responseText = await response.text();
                console.log('Upload response:', responseText);

                if (!response.ok) {
                    throw new Error(`Upload failed: ${responseText}`);
                }

                successCount++;
                showMessage(`Successfully uploaded ${file.name}`);
            } catch (error) {
                console.error('Upload error:', error);
                failCount++;
                showMessage(`Failed to upload ${file.name}: ${error.message}`, true);
            }
        }

        if (successCount > 0) {
            showMessage(`Successfully uploaded ${successCount} file(s)`);
        }
        if (failCount > 0) {
            showMessage(`Failed to upload ${failCount} file(s)`, true);
        }

        await updateDocCount();

        hideLoading();
        progressDiv.style.display = 'none';
        fileInput.value = '';
    };

    fileInput.addEventListener('change', (e) => {
        console.log('File input changed');
        const files = e.target.files;
        if (files.length > 0) {
            console.log('Files selected:', files.length);
            handleFileUpload(files);
        }
    });

    searchForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const query = searchInput.value.trim();
        if (!query) return;

        showLoading();
        try {
            const response = await fetch(`/api/search?q=${encodeURIComponent(query)}`);
            if (!response.ok) throw new Error('Search failed');
            
            const data = await response.json();
            
            if (!data.results || data.results.length === 0) {
                resultsDiv.innerHTML = '<div class="no-results">No results found</div>';
                return;
            }

            resultsDiv.innerHTML = data.results.map(result => `
                <div class="result-item">
                    <h3>${result.path || 'Untitled Document'}</h3>
                    <div class="meta">
                        Type: ${result.type || 'Unknown'} | 
                        Indexed: ${new Date(result.indexed).toLocaleString()}
                    </div>
                    ${result.snippets.map(snippet => `
                        <div class="snippet">${snippet}</div>
                    `).join('')}
                    <div class="actions">
                        <a href="${result.download_url}" target="_blank">Download</a>
                        <a href="${result.view_url}" target="_blank">View</a>
                    </div>
                </div>
            `).join('');
        } catch (error) {
            resultsDiv.innerHTML = `<div class="error">Search failed: ${error.message}</div>`;
        } finally {
            hideLoading();
        }
    });

    updateDocCount();
}); 