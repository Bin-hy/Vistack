import { defineCustomElement } from "vue";
import myButton from "./components/my-button.vue"

// 注册自定义元素——原写法
// const MyButtonElement =  defineCustomElement(myButton);
// customElements.define("vsk-button", MyButtonElement)

const components = [
  {name: "vsk-button", comp: myButton}
]

components.forEach((item) => {
  const element = defineCustomElement(item.comp)
  customElements.define(item.name, element)
})
