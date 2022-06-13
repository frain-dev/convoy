import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ForgotPasswordComponent } from './forgot-password.component';
import { RouterModule, Routes } from '@angular/router';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';

const routes: Routes = [{ path: '', component: ForgotPasswordComponent }];

@NgModule({
	declarations: [ForgotPasswordComponent],
	imports: [CommonModule, ReactiveFormsModule, FormsModule, RouterModule.forChild(routes)]
})
export class ForgotPasswordModule {}
