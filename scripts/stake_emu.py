#!/usr/bin/env python3
# -*- coding: utf-8 -*-
from collections import defaultdict

EpochsInHour = 120
EpochsInDay = 2880
FIL_PRECISION = 10**18


class RunTime(object):
    def __init__(self):
        self.epoch = 0
        self.caller = ""
        self.amount = 0


class VestingSpec(object):
    def __init__(self, vest_period, step_duration):
        self.step_duration = step_duration
        self.vest_period = vest_period
        self.initial_delay = 0
        self.quantization = 12 * EpochsInHour


class VestingFunds(object):
    def __init__(self):
        self.funds = []

    def unlock_vested_funds(self, curr_epoch):
        unlocked = 0
        last_index_to_rm = -1
        for i, (epoch, amount) in enumerate(self.funds):
            if epoch >= curr_epoch:
                break
            unlocked += amount
            last_index_to_rm = i
        if last_index_to_rm != -1:
            self.funds = self.funds[last_index_to_rm+1:]
        return unlocked

    def quantize_up(self, e, unit, offset_seed):
        offset = offset_seed % unit
        remainder = (e - offset) % unit
        quotient = (e - offset) // unit
        if remainder == 0:
            return unit * quotient + offset
        if (e - offset) < 0:
            return unit * quotient + offset
        return unit * (quotient + 1) + offset

    def add_locked_funds(self, curr_epoch, vesting_sum, stake_period_start, vest_spec: VestingSpec):
        epoch_to_index = {}
        for i, (epoch, amount) in enumerate(self.funds):
            epoch_to_index[epoch] = i
        vest_begin = curr_epoch + vest_spec.initial_delay
        vested_so_far = 0
        e = vest_begin + vest_spec.step_duration
        while vested_so_far < vesting_sum:
            vest_epoch = self.quantize_up(e, vest_spec.quantization, stake_period_start)
            elapsed = vest_epoch - vest_begin
            if elapsed < vest_spec.vest_period:
                target_vest = vesting_sum * elapsed // vest_spec.vest_period
            else:
                target_vest = vesting_sum
            vest_this_time = target_vest - vested_so_far
            vested_so_far = target_vest

            if vest_epoch in epoch_to_index:
                index = epoch_to_index[vest_epoch]
                epoch, amount = self.funds[index]
                self.funds[index] = (epoch, amount+vest_this_time)
            else:
                self.funds.append((vest_epoch, vest_this_time))
                epoch_to_index[vest_epoch] = len(self.funds) - 1
            e += vest_spec.step_duration
        self.funds = sorted(self.funds, key=lambda x: x[0])


class StakeActor(object):

    def __init__(self, round_period, principal_lock_duration, mature_period, max_reward_per_round, inflation_factor, next_round_epoch, vest_spec):
        self.round_period = round_period
        self.principal_lock_duration = principal_lock_duration
        self.mature_period = mature_period
        self.max_reward_per_round = max_reward_per_round
        self.inflation_factor = inflation_factor
        self.stake_period_start = next_round_epoch
        self.next_round_epoch = next_round_epoch
        self.vest_spec = vest_spec
        self.total_stake_power = 0
        self.last_round_reward = 0
        self.inflation_denominator = 10000
        self.locked_principal_map = defaultdict(list)
        self.available_principal_map = defaultdict(int)
        self.vesting_reward_map = defaultdict(VestingFunds)
        self.available_reward_map = defaultdict(int)
        self.stake_power_map = defaultdict(int)

    def deposit(self, rt: RunTime):
        self.locked_principal_map[rt.caller].append((rt.epoch, rt.amount))

    def withdraw_principal(self, rt: RunTime):
        amount = rt.amount
        avail = self.available_principal_map[rt.caller]
        if amount <= avail:
            self.available_principal_map[rt.caller] -= amount
        else:
            print("!:", rt.epoch, "error withdraw_principal more than available")

    def withdraw_reward(self, rt: RunTime):
        amount = rt.amount
        avail = self.available_principal_map[rt.caller]
        if amount <= avail:
            self.available_principal_map[rt.caller] -= amount
        else:
            print("!:", rt.epoch, "error withdraw_reward more than available")

    def unlock_locked_principals(self, rt: RunTime):
        for staker, locked_principals in self.locked_principal_map.items():
            unlocked = 0
            last_index_to_rm = -1
            for i, (epoch, amount) in enumerate(locked_principals):
                if epoch + self.principal_lock_duration >= rt.epoch:
                    break
                unlocked += amount
                last_index_to_rm = i
            if last_index_to_rm != -1:
                self.locked_principal_map[staker] = locked_principals[last_index_to_rm+1:]
                self.available_principal_map[staker] += unlocked

    def update_stake_powers(self, rt: RunTime):
        total = 0
        powers = defaultdict(int)
        for staker, locked_principals in self.locked_principal_map.items():
            power = 0
            for (epoch, amount) in locked_principals:
                if epoch + self.mature_period >= rt.epoch:
                    break
                power += amount
            powers[staker] = power
            total += power
        for staker, available_principal in self.available_principal_map.items():
            powers[staker] += available_principal
            total += available_principal
        self.stake_power_map = powers
        self.total_stake_power = total

    def unlock_vesting_rewards(self, rt: RunTime):
        for staker, vesting_funds in self.vesting_reward_map.items():
            unlocked = vesting_funds.unlock_vested_funds(rt.epoch)
            self.vesting_reward_map[staker] = vesting_funds
            self.available_reward_map[staker] += unlocked

    def distribute_rewards(self, rt: RunTime) -> int:
        assert rt.epoch >= self.next_round_epoch
        total_reward = 0
        vest_spec = self.vest_spec

        if self.total_stake_power > 0:
            total_reward = self.total_stake_power * self.inflation_factor // self.inflation_denominator
            total_reward = min(total_reward, self.max_reward_per_round)
            if total_reward > 0:
                for staker, power in self.stake_power_map.items():
                    vesting_sum = power * total_reward // self.total_stake_power
                    if vesting_sum > 0:
                        vesting_funds = self.vesting_reward_map[staker]
                        vesting_funds.add_locked_funds(rt.epoch, vesting_sum, self.stake_period_start, vest_spec)
        return total_reward

    def on_epoch_tick(self, rt: RunTime):
        self.unlock_locked_principals(rt)
        self.update_stake_powers(rt)
        self.unlock_vesting_rewards(rt)
        if rt.epoch >= self.next_round_epoch:
            self.last_round_reward = self.distribute_rewards(rt)
            self.next_round_epoch += self.round_period


class Message(object):
    def __init__(self, epoch: int, sender: str, amount: int, func):
        self.epoch = epoch
        self.sender = sender
        self.amount = amount
        self.func = func


class VM(object):
    def __init__(self, stake_actor: StakeActor):
        self.stake_actor = stake_actor

    def exec(self, messages: list[Message], stop_at: int):
        rt = RunTime()
        message_map = defaultdict(list[Message])
        for msg in messages:
            message_map[msg.epoch].append(msg)

        for epoch in range(0, stop_at + 1):
            rt.epoch = epoch
            for msg in message_map[epoch]:
                rt.caller = msg.sender
                rt.amount = msg.amount
                msg.func(rt, self.stake_actor)
            rt.caller = "system"
            rt.amount = 0
            self.stake_actor.on_epoch_tick(rt)


def run():
    stake_actor = StakeActor(
        round_period=10,
        principal_lock_duration=1,
        mature_period=1,
        max_reward_per_round=30000*FIL_PRECISION,
        inflation_factor=100,
        next_round_epoch=13,
        vest_spec=VestingSpec(180*EpochsInDay, EpochsInDay)
    )
    vm = VM(stake_actor)
    messages = []
    messages.append(Message(epoch=14, sender="t001", amount=10000*FIL_PRECISION, func=lambda rt, actor: actor.deposit(rt)))
    vm.exec(messages, stop_at=25)
    print("locked_principal_map", stake_actor.locked_principal_map)
    print("available_principal_map", stake_actor.available_principal_map)
    print("stake_power_map", stake_actor.stake_power_map)
    print("total_stake_power", stake_actor.total_stake_power)
    print("vesting_reward_map", stake_actor.vesting_reward_map["t001"].funds)


if __name__ == "__main__":
    run()