import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import {resolve} from "path"

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue({customElement: true})],

  // https://cn.vite.dev/config/build-options.html#build-lib
  build: {
    lib:{
      entry: resolve(__dirname, "src/main.ts"),
      fileName:`vsk-web-components.js`,
      formats: ['es'] // es模块格式
    },
    rollupOptions:{
      // 不打包 Vue runtime
      external: ['vue'],
    }
  }
})
