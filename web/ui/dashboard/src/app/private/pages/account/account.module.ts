import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AccountComponent } from './account.component';
import { RouterModule, Routes } from '@angular/router';
import { ReactiveFormsModule } from '@angular/forms';
import { PageComponent } from 'src/app/components/page/page.component';
import { InputComponent } from 'src/app/components/input/input.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { CardComponent } from 'src/stories/card/card.component';

const routes: Routes = [{ path: '', component: AccountComponent }];

@NgModule({
	declarations: [AccountComponent],
	imports: [CommonModule, ReactiveFormsModule, RouterModule.forChild(routes), PageComponent, InputComponent, ButtonComponent, CardComponent]
})
export class AccountModule {}
