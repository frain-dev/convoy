import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AccountComponent } from './account.component';
import { RouterModule, Routes } from '@angular/router';
import { ReactiveFormsModule } from '@angular/forms';

const routes: Routes = [{ path: '', component: AccountComponent }];

@NgModule({
	declarations: [AccountComponent],
	imports: [CommonModule, ReactiveFormsModule, RouterModule.forChild(routes)]
})
export class AccountModule {}
