import App from './App.svelte';
import './styles.css';
import { mount } from 'svelte';

const target = document.getElementById('app');

if (!target) {
  throw new Error('App mount target #app was not found');
}

const app = mount(App, { target });

export default app;
