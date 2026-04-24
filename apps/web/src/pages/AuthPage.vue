<template>
  <q-page class="flex flex-center">
    <q-card style="min-width: 360px">
      <q-card-section>
        <div class="text-h6">Вход</div>
      </q-card-section>

      <q-card-section class="q-pt-none">
        <q-form @submit="handleSignIn" class="q-gutter-md">
          <q-input
            v-model="email"
            label="Email"
            type="email"
            :rules="[(val) => !!val || 'Обязательное поле']"
          />
          <q-input
            v-model="password"
            label="Пароль"
            type="password"
            :rules="[(val) => !!val || 'Обязательное поле']"
          />
          <q-btn label="Войти" type="submit" color="primary" class="full-width" />
        </q-form>
      </q-card-section>
    </q-card>
  </q-page>
</template>

<script setup lang="ts">
import { ref } from "vue";

const email = ref("");
const password = ref("");

function handleSignIn() {
  // TODO: вызов useAuthStore().signIn(email.value, password.value)
}
</script>
