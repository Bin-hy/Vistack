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
      outDir: resolve(ProjectRoot, "build/web-client",)
    },
    base: './',
    resolve: {
      alias: {
          '@': resolve("./src") //fileURLToPath(new URL('./src', import.meta.url))
      }
    },
      define: {
        'process.env': env
      }
    })
}
