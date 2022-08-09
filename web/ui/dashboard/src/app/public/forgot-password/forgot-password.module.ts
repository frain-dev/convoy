import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ForgotPasswordComponent } from './forgot-password.component';
import { RouterModule, Routes } from '@angular/router';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { InputComponent } from 'src/app/components/input/input.component';

const routes: Routes = [{ path: '', component: ForgotPasswordComponent }];

@NgModule({
	declarations: [ForgotPasswordComponent],
	imports: [CommonModule, ReactiveFormsModule, FormsModule, ButtonComponent, InputComponent, RouterModule.forChild(routes)]
})
export class ForgotPasswordModule {}
