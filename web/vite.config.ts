import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '0.0.0.0', // 监听全部网卡，局域网设备可通过本机 IP 访问（如 http://192.168.x.x:28170）
    port: 28170,
    // 开发模式代理到后端，避免跨域
    proxy: {
      '/api': {
        target: 'http://localhost:28180',
        changeOrigin: true,
      },
    },
  },
})
