# 重卡充电站运营监控台

前端使用 Vue 3 + Vite + ECharts，后端使用 Go + MySQL。页面直接读取：

- `site_exploration_charging_station_result`：站点最新电价、功率、分时费率和逐桩明细
- `site_exploration_charging_station_dynamic_history`：按采集轮次生成的历史快照

## 启动后端

后端默认从当前目录或上层目录查找 `.env` 中的 `EVCS_DATABASE_URL`。

```bash
cd backend
go mod tidy
go run .
```

默认监听 `http://127.0.0.1:8088`。

## 启动前端

```bash
cd frontend
npm install
npm run dev
```

默认地址为 `http://127.0.0.1:5173`，Vite 会把 `/api` 代理到 Go 后端。

## API

- `GET /api/health`
- `GET /api/overview`
- `GET /api/map-stations`
- `GET /api/stations?page=1&pageSize=25&sort=priceAsc`
- `GET /api/station?sourceKey=...`
- `GET /api/station/history?sourceKey=...`
- `GET /api/meta`

列表接口只返回页面需要的汇总字段，完整 `result_payload` 只在站点详情接口中返回，避免一次性把 2500 多条 JSON 全部传到浏览器。

## 右侧当前筛选概览

未选中站点时，右侧显示空闲分布、低价可充站 TOP 5 和数据新鲜度；选中站点显示详情，关闭详情返回概览。

- `/api/overview` 供顶部全局指标使用，不随左侧筛选变化。
- `/api/stations` 的 `summary` 覆盖全部筛选结果，分页仅影响 `items`。空闲状态筛选参数为 `availability=idle|moderate|full|unknown`，阈值与站点卡片一致（≥50%、20%–50%、<20%、无电桩数据）。
- TOP 5 只包含有电价且有空闲桩的站点；按电价升序，同价优先空闲桩多的站点。
- 新鲜度按 `capturedAt` 计算，24 小时内为近期采集，超过 24 小时为待更新；缺失、无效或未来时间为未知。`receivedAt` 和页面刷新时间不作为采集时间的替代。
- 修改 Go 后端后需重新编译并重启进程，Vite 热更新只更新前端。若提示“站点接口缺少概览数据”，请检查正在运行的后端是否为新版本。

浏览器回归测试（启动前端开发服务后打开）：
- `/tests/station-card-layout.html`：卡片、骨架、主题布局。
- `/tests/filter-auto-refresh.html`：筛选即时更新、搜索防抖、中文输入及重置交互。

## 服务器部署

推荐目录为 `/opt/evcs-station-dashboard`。服务器需要安装 Go 1.22+、Node.js 20+、Nginx。

```bash
sudo mkdir -p /opt/evcs-station-dashboard
sudo chown -R "$USER":"$USER" /opt/evcs-station-dashboard
git clone https://github.com/hjw-alt/evcs-station-dashboard.git /opt/evcs-station-dashboard
cd /opt/evcs-station-dashboard
```

### 1. 构建后端

```bash
cd /opt/evcs-station-dashboard/backend
go mod download
go build -o station-dashboard .
cp ../deploy/backend.env.example .env
```

编辑 `.env`，填写远程 MySQL 地址。生产环境建议把 `DASHBOARD_ALLOWED_ORIGIN` 设置为实际访问域名。

```bash
chmod 640 .env
chown root:www-data .env
sudo cp ../deploy/station-dashboard.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now station-dashboard
sudo systemctl status station-dashboard
```

### 2. 构建前端

```bash
cd /opt/evcs-station-dashboard/frontend
npm ci
npm run build
```

### 3. 配置 Nginx

```bash
sudo cp ../deploy/nginx-station-dashboard.conf /etc/nginx/conf.d/station-dashboard.conf
sudo nginx -t
sudo systemctl reload nginx
```

访问 `http://服务器公网IP/`。页面接口统一走同域 `/api/`，不需要在前端写死服务器地址。

### 排查

```bash
curl http://127.0.0.1:8088/api/health
journalctl -u station-dashboard -n 100 --no-pager
sudo tail -f /var/log/nginx/error.log
```
