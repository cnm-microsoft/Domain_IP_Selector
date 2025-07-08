document.addEventListener('DOMContentLoaded', () => {
    const form = document.getElementById('config-form');
    const runTestBtn = document.getElementById('run-test');
    const saveConfigBtn = document.getElementById('save-config');
    const progressLog = document.querySelector('#progress-log pre');

    let currentConfig = {};
    let locationsData = {};

    // --- WebSocket Logic ---
    let socket;

    function connectWebSocket() {
        socket = new WebSocket(`ws://${window.location.host}/ws/run`);

        socket.onopen = () => {
            console.log('WebSocket connected');
            progressLog.textContent = 'WebSocket 已连接，准备开始测试...\n';
        };

        socket.onmessage = (event) => {
            const message = event.data;
            progressLog.textContent += `${message}\n`;
            progressLog.scrollTop = progressLog.scrollHeight; // Auto-scroll
        };

        socket.onclose = () => {
            console.log('WebSocket disconnected');
            progressLog.textContent += '\n测试完成，WebSocket 已断开。\n';
        };

        socket.onerror = (error) => {
            console.error('WebSocket error:', error);
            progressLog.textContent += `\nWebSocket 错误: ${error}\n`;
        };
    }

    // --- UI Generation ---
    function createFormGroup(id, label, type = 'number', options = {}) {
        const group = document.createElement('div');
        group.className = 'form-group';

        const labelEl = document.createElement('label');
        labelEl.setAttribute('for', id);
        labelEl.textContent = label;
        group.appendChild(labelEl);

        if (type === 'select') {
            const selectEl = document.createElement('select');
            selectEl.id = id;
            options.choices.forEach(choice => {
                const optionEl = document.createElement('option');
                optionEl.value = choice.value;
                optionEl.textContent = choice.text;
                selectEl.appendChild(optionEl);
            });
            group.appendChild(selectEl);
        } else if (type === 'tags') {
             const container = document.createElement('div');
             container.id = id;
             container.className = 'tag-container';
             group.appendChild(container);
        } else {
            const inputEl = document.createElement('input');
            inputEl.type = type;
            inputEl.id = id;
            if (options.disabled) {
                inputEl.disabled = true;
            }
            group.appendChild(inputEl);
        }
        
        if (options.description) {
            const descEl = document.createElement('p');
            descEl.className = 'description';
            descEl.textContent = options.description;
            group.appendChild(descEl);
        }

        return group;
    }

    function renderForm(config, locations) {
        form.innerHTML = ''; // Clear existing form

        // Concurrency settings (disabled)
        form.appendChild(createFormGroup('dns_concurrency', 'DNS 并发数', 'number', { disabled: true, description: '高并发设置，通常无需修改。如需更改，请直接编辑 config.yaml 文件。' }));
        form.appendChild(createFormGroup('latency_test_concurrency', '延迟测试并发数', 'number', { disabled: true, description: '高并发设置，通常无需修改。如需更改，请直接编辑 config.yaml 文件。' }));
        form.appendChild(createFormGroup('speedtest_concurrency', '速度测试并发数', 'number', { disabled: true, description: '高并发设置，通常无需修改。如需更改，请直接编辑 config.yaml 文件。' }));

        // Other settings
        form.appendChild(createFormGroup('max_latency', '最大延迟 (ms)'));
        form.appendChild(createFormGroup('top_n_per_group', '每组优选 IP 数'));
        form.appendChild(createFormGroup('speedtest_rate_limit_mb', '速度上限 (MB/s, 0为不限速)'));
        form.appendChild(createFormGroup('ip_version', 'IP 版本', 'select', { choices: [{value: 'ipv4', text: 'IPv4'}, {value: 'ipv6', text: 'IPv6'}] }));
        form.appendChild(createFormGroup('group_by', '分组方式', 'select', { choices: [{value: 'region', text: '按地理区域'}, {value: 'colo', text: '按数据中心'}] }));
        
        // Tag-based filters
        form.appendChild(createFormGroup('filter_regions', '筛选区域 (留空则全选)', 'tags'));
        form.appendChild(createFormGroup('filter_colos', '筛选数据中心 (留空则全选)', 'tags'));

        // Populate form with config values
        for (const key in config) {
            const el = document.getElementById(key);
            if (el && el.type !== 'tags') {
                el.value = config[key];
            }
        }
        
        // Populate tags
        populateTags('filter_regions', Object.keys(locations.Regions), config.filter_regions || []);
        populateTags('filter_colos', Object.keys(locations.Colos), config.filter_colos || []);
    }
    
    function populateTags(containerId, allTags, selectedTags) {
        const container = document.getElementById(containerId);
        container.innerHTML = '';
        allTags.sort().forEach(tag => {
            const btn = document.createElement('button');
            btn.className = 'tag-btn';
            btn.textContent = tag;
            btn.dataset.tag = tag;
            if (selectedTags.includes(tag)) {
                btn.classList.add('selected');
            }
            btn.addEventListener('click', (e) => {
                e.preventDefault();
                btn.classList.toggle('selected');
            });
            container.appendChild(btn);
        });
    }

    function getFormData() {
        const config = {};
        const formElements = form.querySelectorAll('input, select');
        formElements.forEach(el => {
            if (!el.disabled) {
                 if (el.type === 'number') {
                    config[el.id] = parseFloat(el.value) || 0;
                } else {
                    config[el.id] = el.value;
                }
            }
        });
        
        // Get selected tags
        config.filter_regions = Array.from(document.querySelectorAll('#filter_regions .tag-btn.selected')).map(btn => btn.dataset.tag);
        config.filter_colos = Array.from(document.querySelectorAll('#filter_colos .tag-btn.selected')).map(btn => btn.dataset.tag);

        return config;
    }


    // --- API Calls ---
    async function loadInitialData() {
        try {
            const [configRes, locationsRes] = await Promise.all([
                fetch('/api/config'),
                fetch('/api/locations')
            ]);
            currentConfig = await configRes.json();
            locationsData = await locationsRes.json();
            renderForm(currentConfig, locationsData);
        } catch (error) {
            console.error('Failed to load initial data:', error);
            alert('加载初始配置失败，请检查程序日志。');
        }
    }

    // --- Event Listeners ---
    runTestBtn.addEventListener('click', () => {
        progressLog.textContent = ''; // Clear log
        connectWebSocket();
        // Use a short timeout to ensure socket is ready before sending
        setTimeout(() => {
            if (socket && socket.readyState === WebSocket.OPEN) {
                const configForTest = getFormData();
                socket.send(JSON.stringify(configForTest));
            } else {
                 progressLog.textContent = 'WebSocket 连接失败，无法开始测试。\n';
            }
        }, 500);
    });

    saveConfigBtn.addEventListener('click', async () => {
        if (!confirm('这将覆盖您现有的 config.yaml 文件，确定要继续吗？')) {
            return;
        }
        try {
            const newConfig = getFormData();
            const response = await fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(newConfig)
            });
            if (response.ok) {
                alert('配置已成功保存！');
                currentConfig = newConfig; // Update local state
            } else {
                const errorText = await response.text();
                throw new Error(errorText);
            }
        } catch (error) {
            console.error('Failed to save config:', error);
            alert(`保存配置失败: ${error.message}`);
        }
    });

    // --- Initial Load ---
    loadInitialData();
});