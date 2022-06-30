import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { LoginComponent } from './login.component';
import { RouterModule, Routes } from '@angular/router';
import { ReactiveFormsModule } from '@angular/forms';
import { InputComponent } from 'src/app/components/input/input.component';
import { ButtonComponent } from 'src/app/components/button/button.component';

const routes: Routes = [
	{
		path: '',
		component: LoginComponent
	}
];

@NgModule({
	declarations: [LoginComponent],
	imports: [CommonModule, RouterModule.forChild(routes), ReactiveFormsModule, InputComponent, ButtonComponent]
})
export class LoginModule {}
