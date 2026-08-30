import { mount } from 'svelte'
import 'bootswatch/dist/darkly/bootstrap.min.css'
import App from './App.svelte'

mount(App, { target: document.getElementById('app')! })
