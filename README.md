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

## 全宽总览与横向筛选

顶部首先展示六个紧凑的全局指标，下面按城市、运营商、空闲状态、电价水平、分时费率分组筛选；点击标签即时更新，关键词搜索防抖 300ms，默认按空闲率降序。城市、运营商可展开全部选项，缺失值以“未标注城市 / 未知运营商”单独列出。

- 取消独立标题栏与左右侧栏，刷新入口合并到筛选工具行，列表、卡片、地图、报告四个 Tab 共用剩余的全宽内容区域；点击站点以弹窗查看详情。
- 城市和运营商在会话内只缓存选项名称（不缓存数量），刷新时显示名称和待更新计数；首次无缓存时显示占位标签。
- `/api/overview` 供顶部六个紧凑的全局指标使用，不随筛选变化。
- `/api/stations` 返回 `facets`：具体选项显示括号数量，“全部”不带数量。每一组的选项计数保留关键词和其他组条件、排除本组条件，便于切换同类选项；数量在分页前统计，不是当前页条数。缺失城市 / 运营商的筛选值为 `__unknown__`。
- `matchedKeys` 包含全部匹配站点的 sourceKey，地图按它过滤有坐标的站点，不受列表分页影响。
- `summary` 覆盖全部筛选结果，分页仅影响 `items`。空闲状态参数为 `availability=idle|moderate|full|unknown`，阈值与站点卡片一致（≥50%、20%–50%、<20%、无电桩数据）。
- 新鲜度按 `capturedAt` 计算，24 小时内为近期采集，超过 24 小时为待更新；缺失、无效或未来时间为未知。`receivedAt` 和页面刷新时间不替代采集时间。
- 报告为文字总览的静态模板，尚未接入大模型；全局电桩指标与筛选范围内站点统计分别标明口径，不将快照统计解释为历史趋势。
- 修改 Go 后端后须重新编译、重启（Vite 仅热更新前端）。提示接口缺少筛选计数或概览数据时，请确认后端运行的是新版本。

浏览器回归测试（启动前端开发服务后打开）：
- `/tests/station-card-layout.html`：列表与卡片骨架随可见区域铺满、缩放/切换/刷新及主题布局。
- `/tests/filter-auto-refresh.html`：动态筛选首屏占位、刷新保留、缓存恢复、零结果/失败、防抖、中文输入及重置交互。

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
