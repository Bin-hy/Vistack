import { defineConfig, loadEnv, type ConfigEnv} from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'
import path from 'path'

export const ProjectRoot = resolve(import.meta.dirname, '../../');
export default ({ mode }: ConfigEnv) => {
  // 手动加载上一级目录的 .env 文件
  const env = loadEnv(mode, path.resolve(__dirname, '../'), '')
  console.log(env.VITE_API_URL)

  return defineConfig({
    plugins: [vue()],
    build: {
      outDir: resolve(ProjectRoot, "build/web-admin",)
    },
    base: './',
    server:{
      port: 8334,
      host: "0.0.0.0"
    },
    resolve: {
      alias: {
          '@': resolve("./src"),
          '@ui': resolve(ProjectRoot, "web/ui/src")
      }
    },
      define: {
        'process.env': env
      }
    })
}
