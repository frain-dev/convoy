import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ResetPasswordComponent } from './reset-password.component';
import { RouterModule, Routes } from '@angular/router';
import { ReactiveFormsModule } from '@angular/forms';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { InputComponent } from 'src/app/components/input/input.component';

const routes: Routes = [{ path: '', component: ResetPasswordComponent }];

@NgModule({
	declarations: [ResetPasswordComponent],
	imports: [CommonModule, ReactiveFormsModule, ButtonComponent, InputComponent, RouterModule.forChild(routes)]
})
export class ResetPasswordModule {}
